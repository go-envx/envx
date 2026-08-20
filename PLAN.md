# Variable Substitution: Compose Values From Other Variables

This plan adds variable substitution to envx: a value may embed a reference to another resolved value, e.g. `POSTGRES_URL: postgresql://user:pass@{{POSTGRES_HOST}}:5432/db`. References may point at other references in any order, transitively, with circular-reference and missing-reference detection. Substitution runs as a reveal-gated stage of resolution, so masked output shows the variable definition and `--reveal` produces the composed value. Each task below is one independently reviewable pull request.

## One effective environment; masked vs revealed

`get`, `explain`, and `run` all read the same effective environment, so their output mirrors each other. Two axes govern it, and they are independent:

- **Source selection (OS override + overload) — always applied, masked or revealed.** A namespace key that is also set in the OS environment resolves to the OS value by default, or to the namespace value under `overload`, exactly as the `run` child process would see it. So masked `get POSTGRES_HOST` returns the OS value when it is set — "return the environment variable instead of the namespace value." This mirrors run's *source choice*. In `explain`, an OS override becomes an `"OS environment"` merge source that shadows the file, so the provenance is visible.
- **Substitution and secret decryption — reveal-gated.** Masked mode shows definitions (`{{POSTGRES_PASSWORD}}`, `secret://group/key`); `--reveal` substitutes every reference and decrypts secrets, producing the actual composed value. So masked mode mirrors run's source selection, and `--reveal` mirrors run's final values.

`get`, `explain`, and `diff` gain a `--overload` flag for this. They enumerate namespace keys only (with OS overriding their values); `run`/`Materialize` additionally union the OS-only keys so the child receives a complete environment.

## Where this lives

`{{VAR}}` is pure namespace resolution, but `{{@VAR}}`, OS override, and overload all need the OS environment. Because `get`/`explain`/`diff` need them too, this is a shared resolution concern, not a run-only one. It lives in `envmerge`, which gains an injected OS-environment snapshot (the `os` package is standard library, already used across the codebase; injecting the snapshot rather than calling `os.Environ()` inside the core keeps envmerge hermetically testable) and an `overload` setting. `overload` and the OS-env composition move out of `runner`: `envmerge.Materialize` becomes the complete effective environment and `runner` becomes a pure exec wrapper that receives a ready environment.

## Design decisions (locked)

- **Two reference syntaxes, one delimiter family.** `{{VAR}}` references an envx namespace variable (a final, flattened, uppercased env key), namespace-only, never the OS environment. `{{@VAR}}` references the effective value of a variable: by default the OS environment wins and falls back to the namespace value; under `overload` the namespace value wins and falls back to the OS environment. `@` is chosen over `$` to avoid colliding with Go/Helm `{{$var}}` template variables that may legitimately appear in `values` files. Both patterns are configurable later (Task 7), so the exact sigil is not load-bearing.
- **Escape hatch.** A leading backslash makes the token a literal: `\{{VAR}}` renders `{{VAR}}`. This mirrors the existing `\secret://` escape and lets a value carry an untouched Go/Helm template.
- **References resolve against the final env-key namespace.** A `{{VAR}}` resolves against the merged, post-affix map of env keys (the same names `get`/`explain` print). With a `prefix`/`suffix`/`namespace_prefix` in play, write the final key name.
- **Substitution is reveal-gated; masked shows the definition.** Masked `get`/`explain`/`diff` show the template literal and undecrypted secret references; `--reveal` (and always `run`) substitutes and decrypts. There is no secret taint or masked-wrapping machinery, because masked output never substitutes a secret into a value.
- **The `explain` TYPE for a composed value is `variable`.** Masked `explain` displays the variable definition, not a resolved value, so the type is `variable`; a referenced secret shows as its own reference. `explain` still reports whether the variable resolves (dry-run status) and shows the composed value only under `--reveal`.
- **Always on, disable via a setting.** A `disable_substitution` setting (default `false`) turns substitution off so values pass through literally (Task 5). Escaping covers the per-value case.
- **Missing reference is fatal.** A `{{VAR}}` naming an undefined key, or a `{{@VAR}}` resolving in neither the OS environment nor the namespace, is a hard error (fail-closed, consistent with a dangling `secret://` reference). `explain` reports it as an error-severity row rather than aborting; `run --ignore-errors` (Task 6) can downgrade it so the child still starts.
- **`KindVariableSubstitution` replaces `KindCommandSubstitution`.** The reserved `Kind` is renamed (value `"variable"`). Running a shell command stays a separate, still-unimplemented future feature.

## Syntax

- `{{VAR}}` — internal reference to another env key's resolved namespace value.
- `{{@VAR}}` — effective value: OS environment then namespace fallback (namespace then OS under overload).
- `\{{...}}` — literal `{{...}}` (escape).
- Whitespace inside the braces is trimmed: `{{ VAR }}` == `{{VAR}}`.
- A value may contain any number of references mixed with literal text.

## Pipeline placement

The current resolution stages are: `merge(env)` produces `mergeState` (unresolved `leafValue`s); `openResolver(reveal)` yields the secrets `ValueResolver`; then `resolveLeaf` dereferences each item and `renderLeafValue` joins lists. Substitution adds two stages:

1. **Merge** produces namespace `leafValue`s (may hold `secret://` refs and `{{ }}` templates).
2. **Secret-resolve + render** each leaf (existing) — masked leaves keep secret references, revealed leaves are decrypted — producing `map[string]string` (may still hold `{{ }}` templates).
3. **Compose the effective environment** (new): overlay the injected OS environment per `overload`. `get`/`explain`/`diff` override namespace key values; `run`/`Materialize` also union OS-only keys.
4. **Substitute** (new, reveal-gated): resolve every `{{ }}` reference over the effective environment, transitively. Skipped in masked mode; `explain` runs it in dry-run mode to compute status.

## The substitution engine

A pure, string-in/string-out core, unit-testable in isolation:

- **Input:** the composed effective symbol table (`map[string]string`), a `getenv func(name string) (string, bool)` seam (injected so tests are hermetic), and the `overload` flag ordering `{{@VAR}}` fallback.
- **Tokenizer:** scans a value into literal spans and reference tokens (internal `{{VAR}}` vs OS `{{@VAR}}`), honoring the backslash escape.
- **Resolver:** a dependency graph walked with DFS + memoization. A `visiting` set detects cycles; a resolved-value cache makes each key resolve once regardless of fan-in. `{{VAR}}` looks up the namespace table; `{{@VAR}}` looks up OS-then-namespace (namespace-then-OS under overload), and a namespace hit re-enters the graph. OS values are opaque leaves.
- **Modes:** a reveal mode that returns composed values, and a dry-run mode for `explain` that reports resolvability (OK / unresolved / circular) without exposing the value.
- **Errors:** missing internal reference (names the referencing key and the missing key), missing OS reference (names the referencing key and the variable), and circular reference (lists the cycle path). Errors never include a value.

## Settings

- `disable_substitution` (bool, default `false` = substitution on) — Task 5. Fits the repo's default-false-bool convention.
- Configurable matching patterns (stretch) — Task 7: an internal-reference pattern and an external/OS-reference pattern, each overriding the built-in default, validated (compiled) at config time.

## Tasks

- [x] **Task 1 — Effective environment, overload centralization, and `runner` simplification.** Inject an OS-environment snapshot into `envmerge`; move `overload` from `runner.Params` into `envmerge.Settings`. Compose the effective environment: `get`/`explain`/`diff` override namespace key values from the OS env (overload-aware), with an OS override recorded as an `"OS environment"` merge source that shadows the file; `run`/`Materialize` also union OS-only keys. `envmerge.Materialize` returns the complete effective environment and `runner` becomes a pure exec wrapper taking a ready env (delete its `buildEnv`/overload logic). Add a `--overload` flag to `get`/`explain`/`diff`. No substitution yet. Acceptance: masked `get`/`explain` reflect OS overrides and overload; `explain` shows the OS-environment source; the `run` child environment is byte-identical; `runner` no longer references overload; `task envx:all` green. (May split into a run/runner PR and a get/explain/diff PR.)

- [x] **Task 2 — Substitution engine (pure core) + `Kind` rename.** Add the tokenizer + dependency-graph resolver as a self-contained unit (effective symbol table + injected `getenv` + `overload` in), with reveal and dry-run modes. Support `{{VAR}}`, `{{@VAR}}` with overload-aware fallback, and the `\{{` escape. Rename `KindCommandSubstitution` to `KindVariableSubstitution` (value `"variable"`). Not wired into any operation yet. Acceptance: exhaustive unit tests (order-independence, transitive chains, diamond fan-in, cycles of length 1 and N, missing internal ref, missing OS ref, OS-then-namespace and overload precedence, escape, multiple refs per value, dry-run status, literal passthrough) and `task envx:all` green.

- [x] **Task 3 — Wire reveal-gated substitution into `Materialize`, `Get`, and `Diff`.** Add stage 4, gated on reveal (always on for `run`/`Materialize`). Masked `get`/`diff` show definitions unchanged; revealed operations substitute over the effective environment. `Get` resolves the requested key's transitive dependency closure when revealing (a dangling ref behind a *referenced* variable blocks the read; unrelated dangling refs still do not). Missing-ref and cycle are fatal on the reveal path. Acceptance: `run`, `get --reveal`, and `diff --reveal` resolve `{{VAR}}` and `{{@VAR}}`; masked output shows definitions; cycles and missing refs fail clearly; non-substitution behavior is byte-identical.

- [ ] **Task 4 — `explain` substitution diagnostics.** Diagnose each composed value in dry-run mode so status is reported even when masked: TYPE `variable`, LITERAL the template as written, STATUS one of OK / `UNRESOLVED_VARIABLE` / `CIRCULAR_REFERENCE`, and the composed value in the RESOLVED column only under `--reveal`. Acceptance: masked `explain` shows templates with resolvability status in table and JSON; the summary banner reflects unresolved/circular entries; `--reveal` shows resolved values; no value leaks in masked mode.

- [ ] **Task 5 — `disable_substitution` setting.** Full precedence plumbing: `schema.DisableSubstitution` FlagSpec (flag `--disable-substitution`, env `ENVX_DISABLE_SUBSTITUTION`), `schema.Settings` + `envmerge.Settings` fields, manifest key, `config.Input`, flag binding on the resolving commands, and `GetInput` wiring. When set, stage 4 is skipped and `{{ }}` tokens pass through literally. Acceptance: setting resolves through the full flag > env > project > global > default chain and disables substitution; default keeps substitution on.

- [ ] **Task 6 — `run --ignore-errors`.** A command-local flag on `run` that downgrades *resolution* errors (missing internal ref, missing OS ref, dangling secret) to stderr warnings and omits the failing keys from the child environment so the process still starts. Structural errors (YAML parse, flatten collision) stay fatal; the cycle-vs-fatal decision is settled in this task. Acceptance: `run --ignore-errors` with a missing reference starts the child with the failing key omitted and a warning on stderr; without the flag it fails; a malformed file still aborts regardless.

- [ ] **Task 7 — Configurable matching patterns (stretch).** Two settings supplying regexes for the internal-reference and external/OS-reference syntaxes, each overriding the built-in default and validated (compiled) at config time with a clear error on an invalid pattern. Acceptance: a workspace can redefine the reference syntaxes; an invalid pattern fails at config resolution, not at use.

- [ ] **Task 8 — Documentation.** Site docs (a substitution page under configuration, plus schema entries for the new setting/flags and the `--overload` additions) and any README/help-text updates, done once the feature is complete and safe to teach.

## Out of scope

- Command substitution (running a shell command); `KindVariableSubstitution` is this feature, and shell command substitution remains a separate future feature.
- A masked structural preview (substitute non-secrets, wrap secrets as `{secret://group/key}`); the definition view is used instead. Revisit only if a safe composed preview is wanted, which would reintroduce secret taint tracking.
- Default-value syntax for references (e.g. `{{@VAR:-fallback}}`); revisit only if a real need appears.
- Re-checking a substituted expansion against the list delimiter.
- Making `diff` ignore OS source selection. `diff` currently applies OS overrides to both sides (matching what `run` would inject), so a namespace key also set in the OS environment resolves to that OS value on both sides and reads as no difference — which can mask a genuine file-level difference between the two environments. Revisit whether `diff` should compare pure file declarations (ignore OS) to better serve exploring how config differs across environments.
- External secret backends and any change to the secret store format.
- Windows case-insensitive environment-variable matching. OS-key composition (and the upcoming `{{@VAR}}` OS-reference lookup) matches keys case-sensitively, so on Windows a namespace key colliding case-insensitively with an OS var (e.g. `PATH` vs `Path`) may miss its override and let Go's `os/exec` case-fold dedup pick the surviving value nondeterministically. This is pre-existing behavior (the former `runner` env merge was equally case-sensitive) and is preserved byte-for-byte here; first-class Windows support would need a platform-aware key-normalization seam applied consistently at every OS-env boundary, done as its own follow-up.
