# envx Diagnostics, Validation, and Emit Design

This document defines the next architecture increment for envx: a diagnostic `explain`, a workspace `validate` command, and an `emit` command. It supersedes the secrets build-out plan, whose local-crypto subsystem is now delivered on `main`. The package stubs describe intended contracts; the Tasks section records what remains. External key providers are intentionally out of scope and deferred to a later plan.

## Overview

### Status

`main` resolves explicit `secret://group/key` references from a workspace `secrets.yaml`, with the full local-crypto subsystem in place: algorithm-neutral ciphers (`internal/cipher`), the private-key boundary (`internal/privatekey`), the nested `envelope` and `store` implementation packages, and an immutable `secrets.Manager` exposing keypair generation, inspection, rotation, single-secret CRUD, and bulk encrypt/decrypt. The `envx keypair` and `envx secrets` command trees are complete, and runtime reveal masks references by default while `run` always reveals and fails closed before starting a child.

Two behaviors motivate this increment. First, `explain` and `diff` still abort on the first unresolved reference through `envmerge.Result.Verify`, so a diagnostic command fails instead of reporting state. Second, there is no `validate` or `emit` command: the workspace has no way to audit every environment at once, and no way to render a resolved environment to an external target.

### Target behavior

- A single dry-run resolution pass classifies each value and reports a non-fatal outcome. It attempts decryption for status but discards materialized plaintext unless reveal is requested, so a diagnostic never leaks secret values and never aborts on a failure.
- `explain` becomes diagnostic. It operates on one project and one environment, reports every key with its type, literal value, source, and status, and exits `0` even when some values fail. When resolution is incomplete it leads with an `ERROR`/`WARNING` summary line so the failure is stated up front.
- `validate` becomes the enforcement command. It aggregates the same dry-run outcomes across every project and every environment, adds workspace store checks, and escalates to a non-zero exit under strict mode. Diagnostic reporting and enforcement share one resolution engine but differ in scope: `explain` is single-environment and always exits `0`; `validate` is all-environment and can fail.
- `emit` resolves an environment and renders it to a delivery target, choosing per target between materializing plaintext through the reveal gate and forwarding references unresolved.
- Resolution outcomes carry a structured severity, a stable code, and a human message. Symbols and color are presentation-only; machine output stays symbol-free and classifiable.
- A value's kind is one of `config_value`, `secret_reference`, or `command_substitution`. Command substitution is reserved: its kind and status codes are defined but not yet produced.

### Security policy

Dry-run resolution is the only new path that touches private keys by default, and it exists to compute status, not to expose plaintext. Materialized plaintext is retained only when reveal is explicitly requested; otherwise it is discarded immediately after the decrypt attempt succeeds or fails. Status codes and messages never include private-key or resolved-secret material, extending the existing redaction rule to the new diagnostic surface. `validate` flags any store value left unencrypted, and `emit` materializes plaintext only for a target and mode whose contract requires it.

### Future constraints

A later base-namespace policy will let a workspace require every key to be declared in its base file; until then `config_value` status is always `ok`, and the `UNDECLARED_IN_BASE` code is reserved for that setting. Command substitution will later populate the `command_substitution` kind and its empty/missing-variable statuses. External key providers and a distinct ordered `secrets.key_sources` field remain deferred to a dedicated provider plan; the v1 `keys_path` field stays a single local filesystem path.

### Resolved policy

- A list value is joined into a single delimiter-separated string for the `VALUE` column, matching what is injected at runtime; `explain` does not display the raw sequence.
- The shared capability is a dry-run resolution. It attempts decryption for status and discards plaintext unless reveal is requested.
- `explain` reports one environment; `validate` spans all environments. Both consume the same per-key dry-run outcomes.
- The next release is a major version, so breaking output changes are acceptable. In particular the `VALUE` column changes meaning from the resolved value to the literal source value, and a new `RESOLVED` column carries plaintext only under reveal.
- `explain` exits `0` and communicates failure through its summary banner and per-row status; enforcement and non-zero exits belong to `validate`.

## Architecture

The stubs below are the intended contracts. Existing symbols keep their current shape where practical; additions are implemented only in the task that needs them.

### `internal/cipher`

`cipher` gains sentinel errors so callers can classify a failed operation into a stable status code instead of matching message text. `Decrypt` returns `ErrDecrypt` when ciphertext cannot be decrypted with the supplied key, and key parsing returns `ErrInvalidKey` for malformed material. `ValidateKeypair` continues to report correspondence failures without exposing private material.

```go
// ErrInvalidKey indicates malformed public or private key material.
var ErrInvalidKey error

// ErrDecrypt indicates ciphertext that could not be decrypted with the given key.
var ErrDecrypt error
```

### `internal/privatekey`

`privatekey` keeps `ErrNotAvailable` as the only "try another source" signal and adds `ErrInvalidKey` for material that is present but malformed. The resolver maps the former to a warning status and the latter to an error status, so diagnostics can distinguish "you simply lack the key here" from "the key you have is broken."

```go
// ErrNotAvailable indicates that no source has a key for the group. It is a
// warning-level outcome: the group's values cannot be revealed here.
var ErrNotAvailable error

// ErrInvalidKey indicates a present but malformed private key. It is an
// error-level outcome distinct from an absent key.
var ErrInvalidKey error
```

### `internal/envmerge`

`envmerge` owns the diagnostic contract because it owns the merged, per-key view of an environment. It defines the resolution outcome DTO and the diagnoser seam it consumes, and adds a `Diagnose` entry point that never aborts. `Build` is unchanged and keeps materializing values for `run` and reveal reads; `Diagnose` records, per key, the literal source value and a non-fatal `Resolution`. The `ValueResolver` seam stays as-is; a resolver opts into diagnostics by also satisfying `ValueDiagnoser`.

```go
// Kind classifies how a value is materialized.
type Kind string

const (
	KindConfigValue         Kind = "config_value"
	KindSecretReference     Kind = "secret_reference"
	KindCommandSubstitution Kind = "command_substitution" // reserved; not yet produced
)

// Severity ranks a resolution outcome.
type Severity string

const (
	SeverityOK      Severity = "ok"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Resolution is the non-fatal, dry-run outcome of materializing one value.
// Resolved carries plaintext only when reveal was requested and succeeded, and
// Code/Message stay free of private-key or secret material.
type Resolution struct {
	Kind        Kind
	Severity    Severity
	Code        string
	Message     string
	Resolved    string
	HasResolved bool
}

// ValueDiagnoser augments a ValueResolver with structured dry-run resolution. It
// classifies a value and reports status without aborting, discarding any
// materialized plaintext unless reveal was requested.
type ValueDiagnoser interface {
	Diagnose(value, environment string) Resolution
}

// Diagnose merges the environment and records a per-key literal value and
// non-fatal Resolution for diagnostic commands. It never aborts on a resolution
// failure; callers inspect Result.Resolution instead of Verify.
func Diagnose(params Params) (*Result, error)

// Literal returns the pre-resolution value written in the winning source file.
func (r *Result) Literal(key string) (string, bool)

// Resolution returns the dry-run outcome recorded for a key.
func (r *Result) Resolution(key string) (Resolution, bool)
```

A key whose value is a joined list aggregates its items into one `Resolution` at the worst observed severity, so a single failing item surfaces without exposing which item. `Build` and `Verify` remain the materialization path for execution; `Diagnose` is the diagnostic path for `explain` and `validate`. `diff` may later adopt `Diagnose` to render a dangling side instead of aborting, but that is outside this plan.

### `internal/secrets`

The resolver implements `envmerge.ValueDiagnoser` in addition to `ValueResolver`, so the same masking-or-revealing resolver also answers dry-run diagnostics. `Diagnose` classifies the value, attempts decryption for a secret reference, and maps the outcome to a `Resolution` using the typed errors above; it populates `Resolved` only when the resolver reveals. The package gains one sentinel error for the dangling case and reuses the cipher and private-key sentinels for the rest.

```go
// ErrSecretNotFound indicates a reference to a value absent from the store.
var ErrSecretNotFound error

// Diagnose reports a value's kind and dry-run resolution without materializing
// plaintext unless the resolver reveals. It never returns an error; a failure is
// reported through the Resolution's severity and code.
func (r *Resolver) Diagnose(value, environment string) envmerge.Resolution
```

This adds a single import edge from `secrets` to `envmerge` for the shared outcome type; `envmerge` does not import `secrets`, so the dependency stays acyclic. The `Manager` and its store, keypair, and encrypt operations are unchanged. The secret-reference status map is:

| Kind | Severity | Code | Meaning |
| --- | --- | --- | --- |
| `secret_reference` | ok | `OK` | decrypts successfully |
| `secret_reference` | warning | `PRIVATE_KEY_UNAVAILABLE` | no private key for the group in this context |
| `secret_reference` | error | `SECRET_NOT_FOUND` | dangling reference; no stored value |
| `secret_reference` | error | `INVALID_PRIVATE_KEY` | key present but malformed or mismatched |
| `secret_reference` | error | `ALGORITHM_MISMATCH` | envelope algorithm unsupported by the cipher |

### `internal/validate`

`validate` is a new engine that aggregates diagnostics across the whole workspace and adds store-level findings that no single environment can see. It enumerates every project and environment, runs `envmerge.Diagnose` for each, and collects reference outcomes; it then inspects the store for values never referenced by any project or environment, values left as plaintext, and keypairs that fail validation. Strict mode escalates warnings to failures. The engine returns a structured report; the action renders it and sets the exit code.

```go
// FindingKind groups a finding by what was inspected.
type FindingKind string

const (
	FindingReference FindingKind = "reference" // per project/environment/key
	FindingOrphan    FindingKind = "orphan"    // stored value never referenced
	FindingPlaintext FindingKind = "plaintext" // store value left unencrypted
	FindingKeypair   FindingKind = "keypair"   // invalid or unavailable identity
)

// Finding is one workspace validation result.
type Finding struct {
	Kind        FindingKind
	Severity    envmerge.Severity
	Code        string
	Message     string
	Project     string // set for reference findings
	Environment string // set for reference findings
	Group       string // set for store findings
	Key         string
}

// Report aggregates findings and whether any reached error severity.
type Report struct {
	Findings []Finding
	Failed   bool
}

// Params configures a workspace validation pass.
type Params struct {
	Strict bool
}

// Validate inspects every project and environment plus workspace store state and
// returns an aggregated report. Strict mode promotes warnings to failures.
func Validate(resolved *config.Result, manager *secrets.Manager, params Params) (Report, error)
```

Orphan detection collects the referenced `(group, key)` set across all projects and environments, then reports any stored secret outside it as a warning. Plaintext and keypair findings reuse the store and `Manager.InspectKeypair` contracts. `validate` never materializes plaintext.

### `internal/emit`

`emit` is a new renderer that writes a resolved environment to a delivery target. It reuses the reveal gate: materialize mode requires successful resolution and emits plaintext, while forward mode emits references unresolved for targets that dereference them downstream. Kubernetes targets split by kind, sending secret-derived values to a `Secret` and the rest to a `ConfigMap`.

```go
// Target names an output format.
type Target string

const (
	TargetDotenv       Target = "dotenv"
	TargetJSON         Target = "json"
	TargetK8sSecret    Target = "k8s-secret"
	TargetK8sConfigMap Target = "k8s-configmap"
)

// Mode selects plaintext materialization or reference forwarding.
type Mode string

const (
	ModeMaterialize Mode = "materialize"
	ModeForward     Mode = "forward"
)

// Params configures one emit render.
type Params struct {
	Target Target
	Mode   Mode
	Name   string // resource/metadata name for Kubernetes targets
}

// Render writes the resolved environment to w in the target format.
func Render(w io.Writer, env *envmerge.Result, params Params) error
```

### `internal/schema`, `internal/config`, and actions

Schema and config add the flags and wiring these commands need without changing the manifest's secret shape. `explain` gains a presentation flag to switch its `SOURCE` column between a path relative to `envx.yaml` (the default) and an absolute path, and keeps its existing `--reveal` and `--output` flags; its status column and summary banner are rendering concerns in the action. `validate` adds an all-environment resolution and a `--strict` flag, so config exposes enough of the manifest's projects and environments for the engine to iterate. `emit` adds target, mode, name, and output-destination flags. No new manifest settings are introduced in this plan; the base-namespace policy and provider sources remain future work.

```go
// ResolveWorkspaceProjects resolves shared paths and dependencies for commands
// that iterate every project and environment, such as validate.
func ResolveWorkspaceProjects(input *Input) (*Result, error)
```

Actions stay thin Cobra adapters: they parse flags, call `envmerge.Diagnose`, `validate.Validate`, or `emit.Render`, and render safely. `explain` reads per-key resolutions instead of calling `Verify`; `validate` owns the only new non-zero diagnostic exit; `emit` writes to stdout or a file.

## Tasks

Each item is one logical PR based on the preceding item after it merges to `main`. A task includes focused tests and leaves no live panic or unreachable placeholder API. The prelude above defines the contracts; each task implements only the additions it needs.

1. ⬜ **Diagnostic resolution and the `explain` redesign:** Introduce the shared dry-run resolution. Add the `cipher` and `privatekey` sentinel errors, the `secrets.ErrSecretNotFound` sentinel, and the resolver's `Diagnose` implementation. Add the `envmerge.Resolution` DTO, the `ValueDiagnoser` seam, and the non-aborting `Diagnose` entry point with per-key literal and resolution accessors. Rebuild `explain` on top of it: replace the `Verify` abort with per-key status, emit `KEY`, `TYPE`, `VALUE` (literal, lists joined), `SOURCE` (relative to `envx.yaml`, with an absolute-path flag), and `STATUS`, add a `RESOLVED` column under `--reveal`, lead with an `ERROR`/`WARNING` summary when resolution is incomplete, and exit `0`. Keep table and JSON output in sync and machine output symbol-free.

2. ⬜ **Workspace validation (`validate`):** Add the `internal/validate` engine and the `envx validate` action. Aggregate `envmerge.Diagnose` outcomes across every project and environment, add orphaned-value, plaintext-store, and invalid-or-unavailable-keypair findings, and report a structured result. Default runs exit `0` with findings; `--strict` promotes warnings to failures and exits non-zero. Reuse the resolver and manager contracts without materializing plaintext, and share the resolution outcome model with `explain`.

3. ⬜ **Emit (`emit`):** Add the `internal/emit` renderer and the `envx emit` action with dotenv, JSON, and Kubernetes `Secret`/`ConfigMap` targets. Support an explicit choice between materializing plaintext through the reveal gate and forwarding references unresolved, split Kubernetes output by secret-derived versus plain values, and write to stdout or a chosen file. Reuse the existing reveal build path so a missing key fails a materialize render before any partial output.

Deferred to a later plan: external key providers and the ordered `secrets.key_sources` field, the base-namespace declaration policy and its `config_value` `UNDECLARED_IN_BASE` status, and the command-substitution feature that will populate the reserved `command_substitution` kind.
