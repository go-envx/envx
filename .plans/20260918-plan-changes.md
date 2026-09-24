# envx v2.0 — Settings Changes Plan

Scope: this plan covers **Item 1 (remove `namespace-prefix`)** and **Item 2 (cut settings keys from `_` to `-`, no alias)**. Item 3 (inline `variables:` in `envx.yaml`) is intentionally deferred pending further discussion and is not planned here. Both items below are breaking changes and are appropriate for the v2.0 boundary.

## Cross-cutting prerequisite — make removed/renamed keys fail loudly

The manifest is decoded with `node.Decode(&manifest)` and no `KnownFields(true)` (see `app/internal/manifest/manifest.go:105-115`), so unknown YAML keys are **silently ignored**. Without addressing this, a v1 manifest that still sets `namespace_prefix:` or `require_overlays:` would parse cleanly and silently drop the setting — the worst outcome for a no-alias break. This must be handled before or alongside Items 1 and 2.

- Decide the mechanism. Option A (recommended): enable strict decoding (`dec := yaml.NewDecoder(bytes.NewReader(data)); dec.KnownFields(true)`) so any unknown key — including the removed/renamed ones — errors at load. Option B: a targeted preflight check that scans the parsed `settings` mappings for a known set of retired keys (`namespace_prefix`, `os_reference_pattern`, `reference_pattern`, `require_overlays`, `keys_path`) and returns a tailored "renamed to `<x>`" / "removed in v2" error.
- Recommendation: strict decoding (A) as the baseline, optionally layered with (B)'s friendlier messages for the specific retired keys so users get a migration hint rather than a bare "unknown field". Strict decoding also catches unrelated typos, which is a general win.
- Caveat to verify: confirm strict decoding is compatible with the two-pass decode (the `yaml.Node` round-trip that preserves indentation for pack's manifest rewrite in `app/internal/pack/manifest.go`). If `KnownFields` cannot be applied cleanly on the node decode, fall back to option B.
- Add tests asserting a manifest using each retired/renamed key produces a clear, actionable error.

## Item 1 — Remove `namespace-prefix`

Rationale: the feature is redundant (nesting one level in a namespace file reproduces the exact same `PARENT_CHILD` output via the `_` join in `toEnvKey`), and it is the root of the pack bug — the prefix is derived from the namespace name (`filepath.Base`), but pack renames namespace files to flattened stems with `-2`/`-3` disambiguation and truncation, so a packed bundle can emit different env-var keys than the source, violating pack's identical-output promise.

### Code to remove

- `app/internal/schema/flagspec.go:121-126` — delete the `NamespacePrefix` `FlagSpec`.
- `app/internal/schema/manifest.go:68-69` — delete the `NamespacePrefix *bool` field and its `yaml:"namespace_prefix"` tag from `Settings`.
- `app/internal/flags/register.go:55-58` — delete `WithNamespacePrefix`.
- `app/internal/flags/input.go:21` — remove `NamespacePrefix: optBool(fs, &schema.NamespacePrefix)` from `GetInput`.
- `app/internal/config/config.go:49-50` — remove the `NamespacePrefix *bool` field from the config/`Input` type; `app/internal/config/config.go:303-306` — remove the `precedenceBool` resolution wiring.
- `app/internal/envmerge/params.go:72-73` — remove the `NamespacePrefix bool` param.
- `app/internal/envmerge/merge.go:153-156` — remove the `if settings.NamespacePrefix { finalKey = toEnvKey(ns.name) + "_" + key }` branch; `finalKey` becomes just `key`.
- Command registrations calling `flags.WithNamespacePrefix`: `app/internal/actions/emit/command.go:125`, `app/internal/actions/diff/command.go:81`, `app/internal/actions/validate/command.go:119`, `app/internal/actions/get/command.go:74`, `app/internal/actions/explain/command.go:90`, `app/internal/actions/run/command.go:79`.

### Tests

- Remove/adjust any test asserting namespace-prefix behavior across `envmerge`, `config`, `flags`, and the affected action packages (grep `NamespacePrefix` / `namespace-prefix` / `namespace_prefix` in `*_test.go`).
- Add a test asserting a manifest that sets `namespace_prefix:` now errors (covered by the cross-cutting prerequisite), confirming it is not silently ignored.

### Fixtures / templates / docs

- `app/testdata/basic/envx.yaml` and any other testdata manifests setting `namespace_prefix` — remove the key (grep confirmed `app/testdata/basic/envx.yaml` and `app/testdata/manifest/valid-secrets/envx.yaml` reference the affected key set).
- `app/internal/actions/create/templates/quick-start/**` — verify the scaffolded workspace does not rely on `namespace-prefix`; if it demonstrates prefixing, convert to the nested-key idiom.
- Docs: remove the `### namespace_prefix` section in `docs/src/content/docs/configuration/schema.mdx:300` (and the example at `:321`), the flag rows in `docs/src/content/docs/commands/{get,diff,run}.mdx`, and the overview entry in `docs/src/content/docs/commands/overview.mdx:41`. Add a short migration note showing the nested-key replacement. Run `task docs:check` since `docs/` changes.

### Verification

- `task envx:check` and `task envx:test`.
- Manual: a workspace with nested keys produces the same output the old `namespace-prefix: true` did; a workspace still setting `namespace_prefix` fails with a clear error.

## Item 2 — Cut settings keys from `_` to `-` (no alias)

Scope clarification (important): the CLI flags are **already** kebab-case (`--namespace-prefix`, `--require-overlays`, `--os-reference-pattern`, `--reference-pattern`), and `ENVX_*` env-var names must stay underscore because shells cannot use dashes. The only surface that is snake_case today is the **YAML manifest keys**. So this item is narrowly about the `yaml:"..."` tags in `app/internal/schema/manifest.go`. After this change the canonical mapping is: flag `--foo-bar` ↔ manifest `foo-bar` ↔ env `ENVX_FOO_BAR`. No alias is retained; old snake_case keys are rejected (see cross-cutting prerequisite).

### Keys to rename (YAML tags in `app/internal/schema/manifest.go`)

- `os_reference_pattern` → `os-reference-pattern` (line 71).
- `reference_pattern` → `reference-pattern` (line 77).
- `require_overlays` → `require-overlays` (line 79).
- `keys_path` → `keys-path` (line 42, under `secrets:`) — see open question below.
- (`namespace_prefix` is deleted by Item 1, not renamed.)
- Already single-word / unaffected: `delimiter`, `env`, `overload`, `prefix`, `suffix`, `path`, `cipher`, `settings`, `environments`, `projects`, `secrets`, `includes`, `validate`.

### Open questions (decide before implementing)

- `keys_path` → `keys-path`: include it for consistency (it is a settings-style key) or leave the `secrets:` block alone? Recommendation: rename it too, so no snake_case remains in the manifest.
- `ValidateSeverities` check-code keys (e.g. `secret_is_not_referenced`) under the `validate:` block are **identifiers/status codes**, not settings, and are matched by `status.Resolve`. Recommendation: leave them as-is (out of scope); renaming them is a separate decision with its own migration surface.

### Tests

- Update every testdata manifest and fixture using the renamed keys to the new spelling (grep `require_overlays|reference_pattern|os_reference_pattern|keys_path` under `app/testdata` — confirmed hits in at least `app/testdata/manifest/valid-secrets/envx.yaml`).
- Update `create` templates under `app/internal/actions/create/templates/quick-start/**` if they use any renamed key.
- Add tests asserting the new kebab keys parse correctly and the old snake keys are rejected with a helpful message.

### Docs

- `docs/src/content/docs/configuration/schema.mdx` — rename the `### require_overlays` heading/anchor (`:455`, `:475`) and any `keys_path`/`reference_pattern`/`os_reference_pattern` examples to kebab; fix in-page anchor links that referenced the old ids.
- `docs/src/content/docs/commands/overview.mdx` and any command pages linking to `#namespace_prefix` / `#require_overlays` anchors — update the anchors.
- Add a v2 migration section listing the renamed keys (old → new). Run `task docs:check`.

### Verification

- `task envx:check` and `task envx:test`.
- Manual: a manifest using the new kebab keys resolves correctly; a manifest using the old snake keys fails with a clear "renamed to `<x>`" error.

## Item 3 — Pack: per-project directory layout

Replace pack's flat bundle (all namespaces flattened into the output root with globally-disambiguated stems) with a per-project directory layout. This is independent of Items 1 and 2 but should land after Item 1, since removing `namespace-prefix` means pack file names no longer affect any resolved value — the redesign then cannot reintroduce the filename-to-env-var coupling class of bug.

### Target layout

- Each selected project gets its own `<project>/` directory in the output. Every file that project includes (base namespace file plus the selected environments' overlays) is copied into `<project>/`, under its original base filename.
- A file shared by two projects is **duplicated** into each project's directory — the "shared file" concept disappears from the bundle. Duplication is intentional and cheap: config files are small and a bundle is a regenerated point-in-time artifact, so there is no edit-drift risk.
- The project subdirectory is **always** present, including single-project packs — uniform structure keeps the manifest-rewrite logic and the consumer's mental model from forking on project count, and reads clearly for debugging.
- One `envx.yaml` at the bundle root; each project's includes are rewritten to `<project>/<basename>`. Consumers still run `envx run --config <out>/envx.yaml --project X`.

### Two-policy contract (make this explicit in code comments and docs)

- Namespace files: duplicated per project directory (no cross-project de-duplication).
- Secrets store: a single shared `secrets.yaml` at the bundle root, filtered to the union of the selected projects' references — pack's current store behavior is kept unchanged.
- Consequence: a multi-project bundle is a convenience/organization artifact, not a per-container isolation artifact — its per-project dirs give organization, collision-scoping, and clean per-project origins, but the store lives at the root, so a project dir is not independently runnable on its own.
- Per-project secret isolation is achieved by re-running pack per project (`envx pack --project X` into a separate output), which already yields a store filtered to just that project. No per-project store-partitioning code is added.

### Code to change

- `app/internal/pack/pack.go` — stop de-duplicating shared namespaces across projects (`discoverIncludes` currently discovers a shared namespace once); discover and copy per project instead. Copy destinations (`copyItem.dest`, `planFiles`) become `<project>/<basename>`. Secrets handling (the single filtered store at root) stays as-is.
- `app/internal/pack/names.go` — collapse the global collision/truncation scheme. Filenames are now preserved within a project directory, so only intra-project base-name collisions need a guard (rare: one project including two different files that share a base name); much of `assignFlatNames`'s global bookkeeping and length-limit truncation can be removed or simplified.
- `app/internal/pack/manifest.go` — `rewriteManifest` maps each project's includes to `<project>/<basename>`, per project rather than through one global stem table.
- `app/internal/pack/references.go` — unchanged; the store stays shared and filtered to the union of selected projects.
- `app/internal/pack/doc.go` — rewrite the package doc: the bundle is no longer flat; describe the per-project layout, the two-policy contract, and the run-per-project isolation guidance.

### Guards

- Project name to directory name: validate/sanitize project names (they are arbitrary `envx.yaml` map keys) for filesystem safety, and guard against a project directory name colliding with a reserved bundle path (the manifest and secrets store stems).
- Intra-project base-name collision: a simple per-directory disambiguation guard, since two includes in one project can still share a base filename.

### Tests

- Update `app/internal/pack/pack_test.go`, `app/internal/pack/names_test.go`, and any layout assertions to the new per-project structure.
- New cases: a file shared by two projects is copied into both directories; the intra-project collision guard fires correctly; a single-project pack still nests under `<project>/`; project-name sanitization; manifest includes rewritten with the project prefix; the secrets store remains a single root file filtered to the union of selected projects.

### Docs

- Pack has no command doc page yet (`docs/src/content/docs/commands/` has no `pack.mdx`). Author `pack.mdx` describing the new layout, the two-policy contract, and the "run pack once per project for per-project secret isolation" note, plus the caveat that a project subdirectory is not independently runnable because the store lives at the bundle root.
- Add pack to `docs/src/content/docs/commands/overview.mdx` if it is not already listed. Run `task docs:check`.

### Verification

- `task envx:check` and `task envx:test`.
- Manual: pack a multi-project workspace and confirm each project directory holds its own copy of every file it includes (shared files duplicated), the root holds one filtered `secrets.yaml` and one rewritten `envx.yaml`, and `envx run --config <out>/envx.yaml --project X` resolves correctly; then pack a single project and confirm the isolated bundle carries a store scoped to only that project.

## Sequencing

1. Land the cross-cutting prerequisite (strict/known-fields decoding + retired-key messaging) — or land it together with Item 1, since Item 1 is the first key to go loud.
2. Item 1 — remove `namespace-prefix`.
3. Item 2 — rename the remaining snake_case manifest keys to kebab.
4. Item 3 — pack per-project directory layout (after Item 1, so pack filenames no longer touch any resolved value).

Each item should be its own PR with green `task envx:check` / `task envx:test` (and `task docs:check` when `docs/` is touched) before merge.

## Note on repo hygiene (not part of these items)

`AGENTS.md` instructs applying the `go-cobra-cli` skill for all `app/` work, but `.agents/skills/go-cobra-cli/SKILL.md` does not exist in the repo (only `web-tester`, `git-commit`, `markdown-editor`, `code-review` are present). Worth adding the skill or fixing the path before the v2 push, since it is meant to be the quality bar for this CLI code.
