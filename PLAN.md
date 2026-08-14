# envmerge Manager Refactor Design

This document defines the refactor that turns `internal/envmerge` from a collection of eager package functions into a configured feature boundary. The package will bind resolved configuration once, defer namespace I/O until an operation is requested, and expose operation-specific methods for single-key reads, diagnostics, environment comparison, and complete materialization. The package stubs describe the target contracts; the Tasks section defines the sequence used to reach them.

## Overview

### Status

`envmerge.Build` currently normalizes `Params`, loads every namespace file, selects winning values, resolves every winner, and returns a general `Result`. Per-key resolution failures are retained in that result so `get` can ignore unrelated failures while `run` and `diff` call `Verify` to require a complete environment. The action packages still own operation semantics around that result: `get` performs key lookup and per-key error selection, `diff` builds two environments and computes their set difference, and `run` decides that verification must succeed before starting a child process.

The diagnostic work in progress adds a separate `envmerge.Diagnose` traversal. It correctly introduces typed resolution outcomes and a non-aborting secrets diagnostic, but it also duplicates the load-and-merge setup from `Build` and extends the general `Result` with fields populated only in diagnostic mode. That mode-dependent result shape is a useful prototype, not the desired endpoint.

### Target behavior

- `envmerge.New` accepts fully resolved `Params`, validates its structural settings and copies them, and returns an immutable `Manager` without reading namespace files, constructing secrets dependencies, opening the secrets store, or constructing a value resolver.
- `Manager.Get` loads and merges the requested environment, selects one winning key, and resolves only that key under the call's reveal policy. Unrelated secret references are never resolved and cannot block the read.
- `Manager.Explain` loads and merges the requested environment, diagnoses every selected winning value without aborting on per-key failures, and returns a dedicated explanation result. Plaintext is retained only when the call explicitly requests reveal.
- `Manager.Diff` loads two environments, compares their rendered literal winners without opening a resolver, and returns a structured comparison. Secret references remain references, so changing a reference is visible even when both references currently decrypt to the same plaintext. The action no longer implements key-set comparison or exposes `--reveal`.
- `Manager.Materialize` loads the requested environment, reveals and resolves every winning value, and returns an environment only when every value succeeds. It aggregates all per-key failures deterministically and never exposes a partial environment.
- Every operation starts from fresh namespace and resolver state. The manager does not cache files, secret-store snapshots, private keys, merged values, resolved values, errors, or plaintext across calls, so repeated calls observe filesystem changes and cannot accidentally reuse secret material.
- Only winning leaves reach a resolver. Shadowed values are loaded for precedence and provenance but are never decrypted or sent to a future external provider.

### Meaning of lazy loading

Lazy loading is operation-level and materialization-level, not file-index-level. `New` performs no namespace I/O. An operation must still read and flatten every included namespace needed for its target environment because a later namespace or overlay can replace an earlier key and because flatten collisions and malformed YAML are operation-fatal. After winner selection, `Get` resolves only its requested winner, `Explain` diagnoses the selected winners, `Materialize` resolves the complete winning set, and `Diff` renders literals without resolving them.

`Diff` is the one multi-environment operation. Within that call it reads and flattens each base namespace once, then applies each side's overlay independently to the operation-local base snapshot. This avoids duplicate base-file work without introducing construction-time or cross-operation caching. A later call still reloads every base and overlay and therefore observes filesystem changes.

The current secrets resolver opens `secrets.yaml` when `config.ResolveProject` constructs it and caches resolved private keys for its lifetime. The target `ValueResolverFactory` corrects both behaviors: config binds a factory containing only secrets and cipher construction params, and each resolving envmerge operation asks it to construct a fresh manager and resolver only after namespace winner selection. The resolver's store snapshot and private-key cache therefore remain operation-scoped; literal-only `Diff` bypasses secrets construction and the factory entirely.

### Security policy

- `New` never loads namespace values, opens a resolver, invokes `ValueResolver.Resolve`, invokes `ValueDiagnoser.Diagnose`, or retains plaintext.
- `Get` can return plaintext only when `GetParams.Reveal` is true; a masked operation opens a masking resolver and returns the canonical reference.
- `Explain` may attempt decryption to classify status, but it stores resolved plaintext only when `ExplainParams.Reveal` is true. A masked explanation carries literals, references, status, and provenance without a materialized value.
- `Materialize` is the explicit fail-closed path for execution and future plaintext exporters. It returns no environment when any winner fails, so `run` cannot pass a partial environment or unresolved reference to a child.
- `Diff` never opens the resolver, reads the secrets store, touches private keys, or retains plaintext. It compares literal winning values after list joining and key transforms.
- Errors, status messages, provenance, and comparison metadata never include private keys or secret plaintext. List errors identify the item index and source key, not the item value.
- Operation-local resolver caches, such as one private-key lookup per group, remain permitted. `Get`, `Explain`, and `Materialize` obtain a fresh resolver from `ValueResolverFactory`, so those caches cannot survive the operation or appear in any result.

### Resolved policy

- The root feature object is named `Manager`, matching `secrets.Manager`: it binds validated dependencies and offers domain operations without exposing mutable state.
- `config` remains the composition boundary. It loads `envx.yaml`, resolves precedence and workspace paths, binds secrets and cipher params into an envmerge-facing resolver factory, and passes a complete `envmerge.Params` to `envmerge.New`. The factory constructs the secrets manager only when invoked. `envmerge` does not parse the manifest or import `config` or `secrets`.
- Actions retain Cobra argument and flag mapping, writer selection, relative-versus-absolute path presentation, table/JSON formatting, and child-process composition. They no longer implement merge-domain workflows.
- Operation-specific results replace the mode-dependent diagnostic fields on `Result`. A materialized environment cannot be mistaken for an explanation, and an explanation does not expose `All`.
- Environment and reveal are operation policy, not manager construction policy. `Settings` contains only merge controls; `Params.DefaultEnvironment` carries the precedence-resolved default, and each operation may select an environment without mutating manager state.
- `Diff` is intentionally literal-only and has no reveal mode. This is a breaking CLI change accepted for the next major release: its `--reveal` flag is removed, and references are compared as declarations rather than by the plaintext they currently produce.
- No cross-operation cache is introduced. CLI commands normally invoke one operation, and freshness plus plaintext lifetime are more valuable than avoiding repeated file reads in hypothetical multi-operation callers.
- The current typed cipher/private-key/secrets errors, `Kind`, `Severity`, `Resolution`, `ValueDiagnoser`, and secrets diagnostic implementation are retained. The standalone `Diagnose(Params)` traversal and diagnostic-only `Result` fields are replaced during migration rather than reverted first.

## Architecture

The stubs below define the target public contract. Because `internal/envmerge` is repository-internal, the migration may remove transitional APIs once every production caller and test has moved; compatibility wrappers exist only to keep intermediate tasks buildable.

### Package layout

```text
internal/envmerge/
    manager.go       # Params, Settings, Manager, New
    materialize.go   # Materialize and complete resolution
    get.go           # Get and Entry
    explain.go       # diagnostic DTOs and Explain
    diff.go          # comparison DTOs and Diff
    merge.go         # namespace loading, winner selection, provenance
    flatten.go       # YAML leaf flattening and rendering
    resolver.go      # resolver interfaces and leaf resolution helpers
    types.go         # Source, Origin, shared enums and DTOs
```

File names describe concepts rather than enforcing a file per method. Existing files may be renamed only when the target ownership is established; unrelated formatting or helper churn is out of scope.

### Public construction and dependencies

```go
// Params supplies resolved namespace paths, settings, and optional value behavior to a Manager.
type Params struct {
    Includes           []string
    Environments       []string
    DefaultEnvironment string
    Settings           Settings
    ResolverFactory    ValueResolverFactory
}

// Settings holds the fully resolved controls used to load and transform an environment.
type Settings struct {
    RequireOverlays  bool
    Prefix           string
    Suffix           string
    Delimiter        string
    NamespacePrefix  bool
}

// ValueResolver resolves one winning scalar or leaves an unrecognized value unchanged.
type ValueResolver interface {
    Resolve(value, environment string) (string, error)
}

// ValueDiagnoser augments a resolver with non-aborting structured diagnosis.
type ValueDiagnoser interface {
    Diagnose(value, environment string) Resolution
}

// ValueResolverFactory opens a fresh operation-scoped resolver on demand.
type ValueResolverFactory interface {
    Resolver(reveal bool) (ValueResolver, error)
}

// Manager binds validated merge configuration without loading namespace files.
type Manager struct {
    params Params
}

// New validates structural settings and privately copies params without performing I/O.
func New(params Params) (*Manager, error)
```

`New` applies structural terminal defaults currently owned by `normalizeParams`, including an empty delimiter becoming `","`, and copies `Includes` and `Environments` so caller mutation cannot change manager behavior. It stores `DefaultEnvironment` without validating it because an explicit operation environment supersedes that default; this prevents an irrelevant `ENVX_ENV` value from blocking `Diff(environmentA, environmentB)`. `normalizeEnvironment` applies the first declared environment when both the call and configured default are empty, then validates the environment actually used by the operation. `New` permits a nil `ResolverFactory`, which remains the identity behavior for plain configuration workspaces. It does not require namespace or secrets files to exist; missing or malformed files are reported by the operation that needs them.

The factory returns a fresh `ValueResolver` for one operation under the requested reveal policy. `Explain` checks whether that resolver also implements `ValueDiagnoser`; `Get` uses `Resolve`; `Materialize` always requests a revealing resolver; and `Diff` never calls the factory. Factory errors are operation-fatal and occur only after namespace loading has produced a valid winning set. Config owns the concrete adapter, which retains `secrets.Params` and `cipher.Params`, calls `NewSecretsManager` only when invoked, then calls `Manager.Resolver` with the reveal policy. This preserves the dependency direction: envmerge defines the consumed interface, while config composes the provider.

### Shared provenance and resolution types

```go
// Source identifies the file and original dotted key that contributed a value.
type Source struct {
    File string
    Key  string
}

// Origin records the winning source and every source it shadowed in merge order.
type Origin struct {
    Winner   Source
    Shadowed []Source
}

// Kind classifies how a value is materialized.
type Kind string

const (
    KindConfigValue         Kind = "config_value"
    KindSecretReference     Kind = "secret_reference"
    KindCommandSubstitution Kind = "command_substitution"
)

// Severity ranks a non-fatal resolution outcome.
type Severity string

const (
    SeverityOK      Severity = "ok"
    SeverityWarning Severity = "warning"
    SeverityError   Severity = "error"
)

// Resolution is the non-fatal outcome of diagnosing one final env-var value.
type Resolution struct {
    Kind        Kind
    Severity    Severity
    Code        string
    Message     string
    Resolved    string
    HasResolved bool
}
```

`Source.File` remains absolute domain data. Relative paths and the `--absolute` flag are presentation choices in the explain action. `Resolution.Resolved` is populated only when the configured resolver reveals and every item in the leaf materializes successfully; `HasResolved` distinguishes an unresolved value from a successfully resolved empty string.

### Single-key lookup

```go
// Entry is one successfully resolved winning env-var value and its provenance.
type Entry struct {
    Key    string
    Value  string
    Origin Origin
}

// GetParams selects one key, environment, and materialization policy.
type GetParams struct {
    Key         string
    Environment string
    Reveal      bool
}

// Get loads one environment and resolves only the requested winning value.
func (m *Manager) Get(params GetParams) (Entry, error)
```

`Get` defaults an empty environment to `Params.DefaultEnvironment`, validates an explicit environment, and normalizes the key to uppercase, preserving the current case-insensitive lookup behavior. It reads and merges all includes to identify the winner, returns `key %q not found` when absent, opens a resolver using `GetParams.Reveal`, and resolves and renders only that winner. Namespace load, flatten, requested-value resolution, and list-rendering failures are returned directly with context. Failures behind unrelated keys are never observed because those leaves never reach the resolver or renderer. Masked get still invokes the resolver so implicit references are canonicalized and escaped references are unescaped; it is not a raw literal read.

### Explanation

```go
// ExplainParams selects all keys or one case-insensitive key from the configured environment.
type ExplainParams struct {
    Key         string
    Environment string
    Reveal      bool
}

// Explanation is a sorted diagnostic view that cannot be consumed as a process environment.
type Explanation struct {
    Entries  []ExplanationEntry
    Summary  ExplanationSummary
}

// ExplanationEntry records one winning literal, its provenance, and its resolution status.
type ExplanationEntry struct {
    Key        string
    Literal    string
    Origin     Origin
    Resolution Resolution
}

// ExplanationSummary aggregates resolution severities across the selected entries.
type ExplanationSummary struct {
    Errors   int
    Warnings int
}

// Severity returns the worst severity represented by the summary.
func (s ExplanationSummary) Severity() Severity

// Explain diagnoses the selected winners without aborting on per-key resolution failures.
func (m *Manager) Explain(params ExplainParams) (*Explanation, error)
```

An empty `ExplainParams.Environment` uses the configured default. An empty `Key` selects every winning key in sorted order; a non-empty key selects one key and returns not-found as an operation error. Namespace/YAML/flatten failures remain fatal because there is no valid merge to explain. Per-value failures are represented by `Resolution` and never abort the operation. The fresh resolver receives `ExplainParams.Reveal`, while diagnosis still attempts decryption when masked so status remains meaningful. Lists aggregate item outcomes at the worst severity, use `secret_reference` when any item is secret-derived, and retain resolved output only when every item materializes.

The explanation action maps `ExplanationEntry` into table and JSON views, converts absolute source paths for presentation, and decides whether to show a `RESOLVED` column. It does not recalculate severity counts or classify resolver errors.

### Complete materialization

```go
// Environment is an immutable, complete set of materialized values and provenance.
type Environment struct {
    values  map[string]string
    origins map[string]Origin
}

// Get returns one materialized value and whether it exists.
func (e *Environment) Get(key string) (string, bool)

// All returns a mutable copy of every materialized key/value pair.
func (e *Environment) All() map[string]string

// Keys returns all materialized keys in sorted order.
func (e *Environment) Keys() []string

// Origin returns one key's provenance and whether it exists.
func (e *Environment) Origin(key string) (Origin, bool)

// Materialize loads and reveals every winner in one environment or returns all failures.
func (m *Manager) Materialize(environment string) (*Environment, error)
```

`Materialize` defaults an empty environment to the configured default, opens a revealing resolver, resolves every winner, records failures by final transformed key, and returns their deterministic sorted `errors.Join` result. It returns a non-nil `Environment` only when no failures occurred; partial values remain operation-local and are discarded on error. This removes the need for callers to remember `Result.Verify` and makes fail-closed execution part of the envmerge contract.

### Environment comparison

```go
// Change records one differing key and its values on each side.
type Change struct {
    Key    string
    Before string
    After  string
}

// DiffResult is the sorted comparison between two complete environments.
type DiffResult struct {
    EnvironmentA string
    EnvironmentB string
    Added        []Change
    Removed      []Change
    Changed      []Change
}

// Diff compares two declared environments using their rendered literal winners.
func (m *Manager) Diff(environmentA, environmentB string) (*DiffResult, error)
```

`Diff` validates both environment names before namespace I/O, loads and flattens every base namespace once for the call, applies each environment's overlays independently, renders each winning literal using the configured delimiter, and compares the two complete literal maps. It never opens a resolver, so dangling references, unavailable private keys, and equal plaintext behind different references do not hide declaration changes or block comparison. Literal means exactly the winning declaration after scalar conversion or list joining: explicit and implicit references are not canonicalized, and a leading escape is not removed. Namespace/YAML/flatten failures remain fatal. Added, removed, and changed slices are sorted by key. The action retains output names such as `env_a` and `env_b` and maps `Before`/`After` into its stable JSON and table formats.

### Key private structures

```go
// namespace identifies one include's base file and environment overlays.
type namespace struct {
    dir  string
    name string
}

// loadedNamespace holds one operation-local parsed base reused across environments.
type loadedNamespace struct {
    namespace
    baseFile string
    baseFlat map[string]leafValue
    baseKeys map[string]string
}

// leafValue preserves scalar-versus-list shape until an operation resolves and renders it.
type leafValue struct {
    items []string
    list  bool
}

// mergeState holds unresolved winning leaves and provenance for one operation and environment.
type mergeState struct {
    values  map[string]leafValue
    origins map[string]Origin
}

// materializedState accumulates complete-resolution values and per-key failures inside one call.
type materializedState struct {
    values  map[string]string
    origins map[string]Origin
    errs    map[string]error
}
```

`mergeState` never contains resolved plaintext. `materializedState` is never embedded into a public result when `errs` is non-empty. The current `resolved` structure is removed rather than expanded further; each public operation result owns only the state guaranteed by its contract.

### Key private functions

```go
// normalizeParams applies terminal defaults, validates settings, and copies caller-owned slices.
func normalizeParams(params Params) (Params, error)

// buildNamespaces converts include paths to stable namespace descriptors without reading files.
func buildNamespaces(includes []string) []namespace

// normalizeEnvironment applies the configured default and validates one operation target.
func (m *Manager) normalizeEnvironment(environment string) (string, error)

// loadNamespaces reads and flattens base files into one operation-local snapshot.
func (m *Manager) loadNamespaces() ([]loadedNamespace, error)

// merge loads one environment, flattens each file independently, and selects unresolved winners.
func (m *Manager) merge(environment string) (*mergeState, error)

// mergeLoaded applies one environment's overlays to an operation-local base snapshot.
func (m *Manager) mergeLoaded(namespaces []loadedNamespace, environment string) (*mergeState, error)

// loadNamespace integrates one base/overlay pair into an operation's merge state.
func loadNamespace(ns loadedNamespace, environment string, settings Settings, state *mergeState) error

// namespaceSources returns one namespace's contributing sources in merge order.
func namespaceSources(key, baseFile, overlayFile string, baseKeys, overlayKeys map[string]string) []Source

// integrateSources updates one final key's winner and complete shadow history.
func integrateSources(state *mergeState, key string, sources []Source)

// applyAffixes applies the global prefix and suffix after namespace winner selection.
func applyAffixes(state *mergeState, settings Settings)

// openResolver obtains fresh operation-scoped value behavior under reveal policy.
func (m *Manager) openResolver(reveal bool) (ValueResolver, error)

// resolveLeaf resolves every item in one winning scalar or list without rendering it.
func resolveLeaf(value leafValue, resolver ValueResolver, environment string) (leafValue, error)

// renderLeaf renders one resolved scalar or joins a list after delimiter validation.
func renderLeaf(value leafValue, sourceKey, delimiter string) (string, error)

// diagnoseLeaf aggregates non-fatal item outcomes without retaining masked plaintext.
func diagnoseLeaf(value leafValue, diagnoser ValueDiagnoser, environment, delimiter string) Resolution

// literalValue renders a winning leaf without invoking a resolver.
func literalValue(value leafValue, delimiter string) string

// materialize resolves every winner and accumulates deterministic per-key failures.
func materialize(state *mergeState, settings Settings, resolver ValueResolver) *materializedState

// materializationError returns all accumulated failures in sorted key order.
func materializationError(errs map[string]error) error

// renderLiterals renders every unresolved winner without invoking a resolver.
func renderLiterals(state *mergeState, delimiter string) (map[string]string, error)

// compare classifies the sorted union of two rendered literal environments.
func compare(environmentA, environmentB string, a, b map[string]string) *DiffResult
```

The existing `loadYAML`, `flatten`, `leafValueFromYAML`, `flattenKeys`, `toEnvKey`, `toMap`, and scalar helpers remain private primitives. `loadNamespace` applies `NamespacePrefix` while each namespace identity is available; `applyAffixes` applies the global prefix and suffix after winners are selected. `merge` creates a fresh base snapshot for an ordinary single-environment operation and delegates to `mergeLoaded`. `Get` calls `merge`, opens one resolver, then resolves one leaf; `Explain` calls `merge`, opens one resolver, then diagnoses selected leaves; and `Materialize` calls `merge`, opens a revealing resolver, then materializes. `Diff` calls `loadNamespaces` once, calls `mergeLoaded` for each side, and renders literals without opening a resolver.

### `internal/config`

`config` remains responsible for manifest discovery, precedence, path resolution, and dependency composition. Its project result exposes a configured manager rather than build parameters once migration is complete.

```go
// Result is the aggregate configuration needed by project and workspace actions.
type Result struct {
    Envmerge *envmerge.Manager
    Runner   runner.Params
    Secrets  secrets.Params
    Cipher   cipher.Params
    // unexported manifest context
}

// ResolveProject loads project configuration, binds a resolver factory, and constructs envmerge.
func ResolveProject(input *Input, project string) (*Result, error)
```

`ResolveProject` resolves environment precedence into `envmerge.Params.DefaultEnvironment` but does not fix an operation's environment or reveal policy. A private config adapter holds `secrets.Params` and `cipher.Params` and implements `envmerge.ValueResolverFactory` by constructing a secrets manager and calling `Manager.Resolver(secrets.ResolverParams{Reveal: reveal})` for each resolving operation. Config then calls `envmerge.New`; neither config nor `New` constructs a cipher, opens `secrets.yaml`, or loads namespace files. `ResolveWorkspace` continues to support workspace-only actions without constructing an envmerge manager. `OverlayPath` and `WorkspaceDir` remain config concerns.

Until the final config migration, `config.Result.Envmerge` remains `envmerge.Params` and each migrated action temporarily calls `envmerge.New(resolved.Envmerge)` before invoking its operation. This keeps one config field and avoids a dual params/manager state. The final config task moves that construction into `ResolveProject` and changes the field to `*envmerge.Manager`; after it lands, actions must not construct managers, reconstruct `Params`, override environment settings, or call compatibility wrappers.

### Actions

The final action flows are intentionally small:

```go
// get
resolved, err := config.ResolveProject(input, project)
entry, err := resolved.Envmerge.Get(envmerge.GetParams{
    Key: key, Environment: environment, Reveal: reveal,
})

// explain
resolved, err := config.ResolveProject(input, project)
explanation, err := resolved.Envmerge.Explain(envmerge.ExplainParams{
    Key: key, Environment: environment, Reveal: reveal,
})

// diff
resolved, err := config.ResolveProject(input, project)
difference, err := resolved.Envmerge.Diff(environmentA, environmentB)

// run
resolved, err := config.ResolveProject(input, project)
environment, err := resolved.Envmerge.Materialize(environmentName)
params := resolved.Runner
params.Env = environment.All()
err = runner.Run(args, params)
```

Actions may define presentation-only DTOs where JSON tags or stable command output require them. They do not duplicate envmerge DTOs merely to rename unexported fields, recompute domain summaries, normalize keys, verify completeness, or compare environments.

## Migration strategy

### Starting workflow

The refactor starts from clean `main`, not from the mixed diagnostic worktree. Before creating the refactor branch, preserve all current tracked, staged, unstaged, and untracked diagnostic work in one named stash. The ignored `.temp/` design files are not included by `-u` and remain available as implementation guidance. Record the resulting stash reference before changing branches.

```sh
git stash push -u -m "wip: diagnostics before envmerge manager refactor"
git stash list -1
```

Do not `stash pop` or apply the complete stash onto the refactor branch. The stash contains three different categories that must land at different times: independent typed-error foundations, manager-incompatible transitional envmerge state, and explain output work that depends on the new manager API. Restore or manually port only the files or hunks assigned to the active task, run that task's focused tests, and leave the remaining stash intact until every reusable piece has either landed or been deliberately superseded.

The current index boundary is not a desired commit boundary. Staged cipher/schema/CLI changes and unstaged envmerge/explain/config/secrets changes are grouped by their target responsibility below, not by whether they happen to be staged today.

### Current-work mapping

| Current work | Migration treatment | Target task |
| --- | --- | --- |
| `cipher/errors.go`, cipher sentinel wiring, and cipher error tests | Restore as an independent typed-error foundation; preserve `%w` classification and existing algorithm behavior. | Task 3 |
| `privatekey.ErrInvalidKey` | Restore with the diagnostic classification work; keep `ErrNotAvailable` distinct. | Task 3 |
| `secrets.ErrSecretNotFound` wrapping | Restore with the diagnostic classification work. | Task 3 |
| `envmerge.Kind`, `Severity`, `Resolution`, and `ValueDiagnoser` | Port into the new manager/type layout; do not restore the old standalone traversal wholesale. | Task 3 |
| `secrets.Resolver.Diagnose` and tests | Adapt to the operation-scoped resolver factory; preserve status codes, redaction, and masked dry-run decryption. | Task 3 |
| `envmerge.Diagnose`, `resolved.literals`, `resolved.resolutions`, and their accessors | Do not restore. Re-express the tested behavior through `Explanation` and `ExplanationEntry` on the shared manager merge kernel. | Superseded by Tasks 1 and 4 |
| Explain action, command, renderer, renderer tests, `schema.Absolute`, `config.WorkspaceDir`, and CLI JSON expectations | Port after `Manager.Explain` exists. Preserve the new output contract while changing its input from mode-dependent `Result` accessors to `Explanation`. | Task 4 |
| `envmerge.applyPrefixSuffix` diagnostic-map handling | Do not restore directly. `applyAffixes` transforms unresolved merge state before operation-specific results are built, so diagnostic maps no longer need special handling. | Superseded by Task 1 |
| `PLAN.md` diagnostics rewrite | Keep stashed while manager PRs are in flight. Reconcile it with the completed manager API after the refactor rather than restoring stale `Build`/`Diagnose` contracts. | Task 8 or a dedicated docs commit |

After the final task, inspect the stash against the completed tree before deleting it. Every remaining hunk should be either already ported, explicitly superseded by the manager design, or intentionally deferred; do not drop the stash merely because the full test suite is green.

### Keep from the current diagnostic work

- `cipher.ErrInvalidKey` and `cipher.ErrDecrypt` plus their algorithm mappings.
- `privatekey.ErrInvalidKey` and the existing `ErrNotAvailable` distinction.
- `secrets.ErrSecretNotFound` and typed error wrapping.
- `envmerge.Kind`, `Severity`, `Resolution`, and `ValueDiagnoser`.
- `secrets.Resolver.Diagnose` and its stable status mapping.
- Explain's new output contract: literal value, type, status, summary, optional resolved value, relative source paths, and `--absolute`.
- Tests that establish diagnostic aggregation, redaction, resolved-empty handling, and status codes.

### Replace during the refactor

- `envmerge.Build(Params)` as the primary entry point becomes `Manager.Materialize`.
- `envmerge.Diagnose(Params)` becomes `Manager.Explain` and no longer duplicates merge setup.
- `Settings.Env` becomes `Params.DefaultEnvironment`, while each manager operation accepts an explicit environment override.
- Reveal moves from `config.ResolveProject` and resolver construction into `GetParams`, `ExplainParams`, and the always-revealing `Materialize` contract.
- `Result.literals`, `Result.resolutions`, `Result.Literal`, and `Result.Resolution` move into `Explanation` and `ExplanationEntry`.
- Deferred `Result.errs`, `Result.Err`, and `Result.Verify` disappear after `Get` and `Materialize` own their distinct error policies.
- `actions/get.runAction`, `actions/diff.buildEnv`, `actions/diff.runAction`, and `actions/diff.unionKeys` move into manager operations or private envmerge helpers.
- Diff's `--reveal` flag and reveal field are removed; comparison uses literal winners and never constructs a resolver.
- Explain's domain summary counting moves into `envmerge.Explain`; path conversion and rendering stay in the action.

### Compatibility wrappers

A temporary wrapper may keep intermediate tasks green after starting from clean `main`:

```go
// Build constructs a temporary Manager and materializes its configured environment.
func Build(params Params) (*Environment, error)
```

No equivalent compatibility wrapper is required for `Diagnose`; Task 4 ports its tests and behavior directly onto `Manager.Explain`. The `Build` wrapper must be deleted in the final cleanup after all production and test call sites use `Manager`; it is not part of the target API.

## Testing strategy

### Manager construction

- `New` applies structural defaults and copies params without validating an environment that may be overridden.
- Each operation defaults and validates the environment it actually uses; explicit diff environments ignore an unrelated configured default.
- `New` copies caller-owned slices.
- `New` succeeds when namespace files do not exist, proving construction performs no namespace I/O.
- `New` never invokes a recording resolver factory, resolver, or diagnoser.
- `Get`, `Explain`, and `Materialize` each open exactly one fresh resolver; `Diff` never opens one.
- A second operation after editing `secrets.yaml` observes the edit and does not reuse private-key cache state.

### Merge kernel

- Base and optional/required overlay behavior remains unchanged.
- Nested and flat spellings across files remain equivalent while same-file collisions still fail.
- Namespace order, overlay order, shadow history, namespace prefixes, prefix, and suffix remain deterministic.
- Shadowed values never invoke the resolver or diagnoser.
- A second operation after editing a namespace file observes the edit, proving no cross-call cache.

### Get

- Lookup is case-insensitive and returns canonical uppercase keys plus provenance.
- An empty environment uses the configured default, and an explicit environment is validated per call.
- Reveal is selected per call; masked references remain canonical and escaped references remain literal.
- Only the requested winner is resolved.
- An unrelated dangling or undecryptable reference does not block the requested key.
- Requested-key resolver and list-rendering failures are returned without leaking values.
- Missing keys return the established error.

### Explain

- All entries are sorted and a selected key is case-insensitive.
- Environment and reveal are selected per call without reconstructing the manager.
- Literals, origins, shadow histories, kinds, statuses, and summary counts are complete.
- Per-key failures do not abort the explanation.
- Masked diagnostics never retain plaintext; reveal preserves a successfully resolved empty string distinctly from unresolved.
- List outcomes aggregate kind and worst severity and resolve only when every item succeeds.
- Paths outside the workspace remain absolute in the action renderer; `filepath.Rel` results beginning with `..` do not escape into displayed relative paths.

### Materialize and run

- Every winner resolves before an environment is returned.
- Materialize always requests a revealing resolver and accepts an explicit or default environment.
- Multiple failures are joined in sorted key order.
- Any failure returns a nil environment and prevents `runner.Run` from starting.
- `All` returns a defensive copy.
- Lists retain delimiter validation and secret-safe errors.

### Diff

- Both environments are validated before file I/O.
- Base files are read and flattened once within the call; each side loads its own overlay without mutating manager state.
- No resolver factory, secrets manager, cipher, secrets store, or private key is touched.
- Added, removed, and changed entries are sorted and carry the expected values.
- Identical environments yield an empty result.
- A namespace or literal-rendering failure on either side returns no partial comparison.
- Different references are reported even when their current plaintext is equal; dangling references compare without error.
- Implicit references and escaped reference literals compare exactly as declared, without resolver canonicalization.
- The command no longer registers or documents `--reveal`.

## Tasks

Each task is one focused change that leaves the tree buildable and includes tests for its new contract. Tasks may be separate commits or PRs depending on review size; no task introduces unused exported APIs or leaves both old and new production paths indefinitely.

1. ⬜ **Introduce the manager, resolver factory, and shared merge kernel:** Add `Manager`, `New`, and `ValueResolverFactory`; add the private config adapter that lazily constructs a fresh secrets manager and resolver for a requested reveal policy; move the precedence-resolved environment from `Settings.Env` to `Params.DefaultEnvironment`; make construction validate/copy params without namespace or secrets construction and defer environment validation to each operation; and consolidate namespace loading and winner selection behind `loadNamespaces`, `merge`, and `mergeLoaded`. Add constructor, freshness, resolver-lifetime, environment, merge, provenance, prefix/suffix, and shadowed-resolver tests. Implement `Materialize(environment)` and `Environment`, keeping `Build` only as a temporary wrapper so existing callers remain green.

2. ⬜ **Move single-key resolution into envmerge:** Add `Entry`, `GetParams`, and `Manager.Get`, resolve only the requested winning leaf under the call's environment and reveal policy, and migrate the get action by temporarily constructing a manager from `config.Result.Envmerge`. Move case normalization, not-found handling, requested-key errors, and provenance selection out of the action. Delete deferred per-key errors from the materialized result once no caller depends on them.

3. ⬜ **Restore typed diagnostic foundations:** Selectively restore the cipher sentinel errors and tests, `privatekey.ErrInvalidKey`, `secrets.ErrSecretNotFound`, `Kind`, `Severity`, `Resolution`, and `ValueDiagnoser`. Adapt `secrets.Resolver.Diagnose` and its tests to the operation-scoped resolver factory while preserving status codes, typed classification, masked dry-run decryption, and plaintext redaction. Do not restore standalone `envmerge.Diagnose` or diagnostic fields on the old result. Run focused cipher, privatekey, secrets, and envmerge contract tests.

4. ⬜ **Fold diagnostics and explain into the manager:** Add `Explanation`, `ExplanationEntry`, `ExplanationSummary`, `ExplainParams`, and `Manager.Explain` on the shared merge kernel, with environment and reveal selected per call. Port the stashed explain action, command, renderer, schema flag, config accessor, CLI expectation, and tests onto `Explanation`; move summary calculation into envmerge; preserve the redesigned output behavior; and leave standalone `Diagnose`, diagnostic fields on `Result`, and their mode-dependent accessors behind. Fix and test outside-workspace source-path fallback in the action as part of the presentation migration.

5. ⬜ **Make diff literal-only inside envmerge:** Add `Change`, `DiffResult`, `Manager.Diff`, operation-local base reuse, literal rendering, and private comparison. Migrate the diff action while preserving its table and JSON shapes, remove `--reveal` from the command and help, and test that references compare without secrets access. Delete the action's `buildEnv`, `runAction`, and `unionKeys` helpers and their action-level domain tests after equivalent envmerge tests exist.

6. ⬜ **Make complete materialization the run contract:** Migrate run to `Manager.Materialize(environment)`, require a complete revealed environment before composing `runner.Params`, and retain the existing child-never-starts-on-resolution-failure test. Remove `Result.Verify`, partial-result exposure, and any remaining direct `Build` production calls.

7. ⬜ **Finalize config ownership and remove compatibility APIs:** Change `config.ResolveProject` to omit reveal, change `config.Result.Envmerge` from `envmerge.Params` to `*envmerge.Manager`, construct it with the default environment and resolver factory, update project-resolution and composition tests, remove the temporary `Build` wrapper and obsolete `Result` type, and verify actions contain only input mapping, cross-package composition, and rendering. Update package documentation to describe the manager lifecycle and lazy operation semantics.

8. ⬜ **Reconcile the stash and run the full review:** Compare the preserved stash against the completed tree, port or explicitly supersede every remaining hunk, and reconcile `PLAN.md` with the final manager API before deleting the stash. Run `task envx:all`, smoke-test masked/revealed get, explain, literal-only diff, and run flows, verify no namespace or secret plaintext is cached across operations, and review the final exported surface for symbols used only by tests. Update diff documentation for the removed flag and literal semantics, then run `docs:check` because this task changes files under `docs/`.

Deferred to a later plan: external resolver backends, context-aware/network resolution, workspace-wide validation, emit targets, and any persistent manager cache or file watcher. The manager API must allow those additions without making config parsing or presentation part of `envmerge`.
