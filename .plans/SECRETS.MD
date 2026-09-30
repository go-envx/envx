# Plan: Secrets Package Refactoring and Clean Architecture Architecture Review

## Context & Objectives

The secrets feature in `app/internal/features/secrets` was recently refactored to align with the Clean Architecture and Dependency Injection roadmap outlined in `.plans/20260924-di-refactor.md`. While the transition from a monolithic, filesystem-coupled manager into consumer-defined interfaces and a dedicated `filestore` subpackage made substantial progress, several organizational and architectural issues remain:

1. **Entity vs. Service Confusion**: Domain records, metadata, service use cases, and evaluation adapters are mixed flat across the package without clear delineations.
2. **Leaked CLI Concerns**: CLI flag specifications (`Group`, `Key`, `NoConfirm`, `Cipher`) are defined in `internal/features/secrets/flags.go` instead of within the presentation adapter (`internal/features/secrets/cli`).
3. **Interface & Scope Incompleteness**: Reference evaluation (`Evaluate`) and diagnosis (`Diagnose`) were moved onto `Resolver`, but their placement, relationship to `Service`, and role in value dereferencing need clarification.
4. **CLI Boilerplate & Deep Nesting**: The CLI subpackages (`decrypt`, `encrypt`, `generate`, `inspect`, etc.) are over-partitioned into nested directories containing redundant `action.go`, `command.go`, `render.go`, `doc.go`, and test files.
5. **Flat Service Methods**: Core service logic is spread across multiple ungrouped files (`encrypt.go`, `keypair.go`, `secret.go`, `inventory.go`, `resolver.go`, `diagnose.go`) making navigation in editor trees difficult.
6. **Migration Strategy**: Determining whether to iterate in place on `internal/features/secrets` or spin up a temporary parallel package like `secrets2` during the transition.

---

## Detailed Review & Answers to Specific Questions

### 1. Delineation of Domain Entities vs. Service Methods

**Current State**:
- Domain records (`SecretRecord`, `SecretReference`, `KeypairRecord`, `KeypairMetadata`, `StoredSecret`) are scattered across `secret.go`, `keypair.go`, `types.go`, and `inventory.go`.
- Methods attached to `*Service` are spread across `secret.go` (`Set`, `Get`, `Delete`), `keypair.go` (`GenerateKeypair`, `InspectKeypair`, `ListKeypairs`, `RotateKeypair`), `encrypt.go` (`Encrypt`, `Decrypt`), `inventory.go` (`StoredSecrets`, `GroupsMissingPublicKey`), and `resolver.go` (`Resolver`).

**Comparison with `features/workspace`**:
- In `workspace/workspace.go`, the domain entity (`Workspace`, `Project`, `Settings`) represents the state and business invariants (`Validate()`, `LookupProject()`, `HasInclude()`). It performs zero external I/O and depends on nothing outside domain primitives.
- `workspace/service.go` defines `Repository` and coordinates loading through `Service.Load()`.

**Recommendation**:
- **Domain Entities & Pure Types** should be grouped into cohesive entity files:
  - `secret.go`: `SecretRecord`, `SecretReference`, and secret validation/normalization functions (`ValidateSecretKey`).
  - `keypair.go`: `KeypairRecord`, `KeypairMetadata`, `PrivateKeyStatus` enum, and group normalization (`NormalizeGroupName`).
  - `reference.go`: `Reference` parsing (`secret://<group>/<key>`), grammar validation.
  - `result.go`: `UpdateResult`, `StoredSecret` (audit model).
- **Domain Service Operations** should live in distinct `service_*.go` files (see point 5).

---

### 2. Location of `flags.go`

**Issue**:
- `internal/features/secrets/flags.go` defines CLI flag definitions using `shared/flags`. Domain packages must never export CLI constructs or depend on CLI concepts.

**Recommendation**:
- Move `flags.go` from `app/internal/features/secrets/flags.go` down to `app/internal/features/secrets/cli/flags.go` (as done in `features/pack/cli/flags.go` and `features/emit/cli/flags.go`).
- Ensure no domain code or other feature imports CLI flags.

---

### 3. Evaluation & Diagnosis: `Resolver` vs. `Service` vs. `SecretRecord`

**Issue Analysis**:
- `Evaluate` and `Diagnose` implement the `value.Evaluator` interface from `shared/value`.
- They operate on dynamic reference strings (e.g. `secret://group/key`) and evaluate environment variables / config entries during interpolation and inspection (e.g. in `envx env explain`, `envx env diff`, and validation).
- Should they live on `SecretRecord`, `Service`, or `Resolver`?
  - **Not on `SecretRecord`**: `SecretRecord` represents one persistent envelope (group, key, algorithm, ciphertext) from storage. Evaluating a raw config string (which may or may not be a secret reference, or may point to a missing secret) is a use-case operation across the store, not a method on an individual secret record.
  - **Why `Resolver` is the right evaluator**: In `envx`, reference evaluation requires a specific policy (masking vs. revealing) and involves caching private keys as groups are encountered. Evaluating 100 references during an `env explain` should not repeatedly hit storage or decrypt unless configured.
  - **How `Service` fits in**: `Service.Resolver(params ResolverParams) (*Resolver, error)` is a factory on the domain service that snapshot-indexes the store and creates an operation-scoped `Resolver` satisfying `value.Resolver` and `value.Evaluator`.
  - **Service Convenience Method**: To keep the service API clean, add a direct facade on `Service` if one-off evaluation is needed:
    ```go
    // Service method for one-off inspection:
    func (s *Service) Evaluate(ref string, reveal bool) (value.Evaluation, error)
    ```
  - **`Diagnose` Deprecation**: `Diagnose` is an alias for `Evaluate` preserved during migration; it should be formally marked deprecated and eliminated once callers migrate to `Evaluate`.

---

### 4. Flattening the CLI Package Architecture

**Current State**:
- Overly granular subdirectories: `cli/decrypt/`, `cli/encrypt/`, `cli/delete/`, `cli/get/`, `cli/set/`, `cli/generate/`, `cli/inspect/`, `cli/print/`, `cli/rotate/`.
- Each contains: `command.go`, `action.go`, `render.go`, `doc.go`, and corresponding `*_test.go` files (over 50 files total).

**Proposed Simplification**:
- Collapsing `action.go` into `command.go` eliminates unnecessary indirection. In modern Clean CLI design, a command runner resolves the service via `core.GetInput` and invokes the service method directly, or delegates the result to a renderer.
- Flatten `cli/` into command pairs grouped by operation:
  - `cli/flags.go`: Shared CLI flags (`--group`, `--key`, etc.).
  - `cli/decrypt_command.go` & `cli/decrypt_command_test.go` (containing command definition, action execution, and flags).
  - `cli/decrypt_output.go` & `cli/decrypt_output_test.go` (or `decrypt_response.go` if focusing on presentation formatting).
  - `cli/encrypt_command.go` & `cli/encrypt_output.go`.
  - `cli/keypair_command.go` (subcommands: generate, inspect, print, rotate).
  - `cli/secrets_command.go` (root group command).
- **Verdict on file naming**:
  - Prefer `[op]_command.go` and `[op]_output.go` (e.g. `decrypt_command.go` and `decrypt_output.go`). "Render" matches the existing codebase conventions (`render.go`, `renderParams`, `printer`) better than "response".

---

### 5. Service Handler File Prefixing

**Proposal**:
- Grouping service methods by capability using a `service_` prefix:
  - `service.go`: `Service` struct definition, `ServiceParams`, constructor `NewService`, consumer-defined interfaces (`Repository`, `CipherClient`, `PrivateKeyService`).
  - `service_crud.go`: `Get`, `Set`, `Delete` methods.
  - `service_bulk.go`: `Encrypt`, `Decrypt` in-place store bulk updates.
  - `service_keypair.go`: `GenerateKeypair`, `InspectKeypair`, `ListKeypairs`, `RotateKeypair`.
  - `service_audit.go`: `StoredSecrets`, `GroupsMissingPublicKey`.
  - `service_resolver.go`: `Resolver` factory, `Resolve`, and `Evaluate` delegation.

**Benefits**:
- Files sort contiguously in file trees (VS Code explorer, `ls`).
- Explicit visual distinction between domain models/entities (`secret.go`, `keypair.go`, `reference.go`) and service behaviors (`service_*.go`).

---

### 6. Migration Strategy: Iterate in Place vs. `secrets2` Package

**Evaluation**:

| Factor | Option A: In-Place Refactoring | Option B: Parallel `secrets2` Package |
|---|---|---|
| **Blast Radius** | Small & Contained: `secrets` is currently only consumed by `internal/core`, `cli/root.go`, and `features/validate`. | High Duplication: Creates parallel duplicate tests, duplicate filestore, and confusing dual types. |
| **Test Coverage** | Existing comprehensive test suite protects against regressions while moving files. | Duplicate test harnesses and mocks must be re-created or copied. |
| **Go Package Semantics** | In Go, moving functions between files within package `secrets` does not break any external imports or callers. | Switching package names forces churn in imports, aliases, and type conversions. |
| **Dependency Cutover** | `features/env` does **not** directly import `features/secrets` (it relies on `value.Resolver` and `value.Evaluator` supplied via `core`). | A parallel package adds no decoupling benefit to `env` since `env` already uses interface-based decoupling. |

**Recommendation**:
- **Refactor in place within `internal/features/secrets`**.
- Because Go packages allow internal file reorganizations without modifying external contracts, you can rename and split files safely while keeping `git` history and test suites intact.
- A `secrets2` package would introduce technical debt, risk stale tests, and require a secondary cleanup pass that is unnecessary given the existing test safety net.

---

## Action Plan & Phased Roadmap

### Phase 1: Clean Domain Boundaries & Relocate Flags
- [x] Move `app/internal/features/secrets/flags.go` to `app/internal/features/secrets/cli/flags.go`.
- [x] Update imports across `app/internal/features/secrets/cli/**` to point to local `flags`.
- [x] Verify `features/secrets` exports zero UI/CLI flags.

### Phase 2: Flatten CLI Subpackages
- [x] Merge `action.go` logic into `command.go` for each CLI operation. Essentially just move the structs and functions from `action.go` as they are into `command.go`.
- [x] Update `render` functions to be named `outputExample` and `renderParams` to be `outputExampleParams`. For example, `outputGet` and `outputGetParams`
- [x] Flatten the nested command folders into `features/secrets/cli/` and rename `render` to `output`:
  - `decrypt_command.go` & `decrypt_output.go`
  - `encrypt_command.go` & `encrypt_output.go`
  - `get_command.go` & `get_output.go`
  - `set_command.go` & `set_output.go`
  - `delete_command.go` & `delete_output.go`
  - `keypair_generate_command.go` & `keypair_generate_output.go`
  - `keypair_inspect_command.go` & `keypair_inspect_output.go`
  - `keypair_rotate_command.go` & `keypair_rotate_output.go`
  - `keypair_print_command.go` & `keypair_print_output.go`
- [x] Eliminate empty subdirectories and unused intermediate actions.
- [x] Consider removing `/ws/app/internal/features/secrets/cli/flags`.
- [x] Update `app/internal/features/secrets/cli/secrets.go` and `cli/keypair.go` command trees.

### Phase 3: Delineate Domain Entities from Service Operations
- [x] Consolidate domain entities:
  - Keep `secret.go` focused purely on `SecretRecord`, `SecretReference`, `PlaintextResolver`, and basic key validation.
  - Keep `keypair.go` focused on `KeypairRecord`, `KeypairMetadata`, and `PrivateKeyStatus`.
  - Keep `reference.go` focused on reference grammar and parsing.
  - Move `UpdateResult` and audit models into `result.go`.
- [x] Rename service method files to follow `service_*.go`:
  - `service_core.go`: `Get` -> `GetSecret`, `Set` -> `SetSecret`, `Delete` -> `DeleteSecret`.
  - `service_crypto.go`: `Encrypt` -> `EncryptSecrets`, `Decrypt` -> `DecryptSecrets`, selection helpers.
  - `service_keypair.go`: `GenerateKeypair`, `InspectKeypair`, `RotateKeypair`, `ListKeypairs`.
  - `service_audit.go`: `StoredSecrets`, `GroupsMissingPublicKey`.
  - `service_resolver.go`: `Resolver` factory, `Evaluate`, `Diagnose`, and resolver mechanics.
- [x] Run `task test` to verify zero functional regressions.

### Phase 4: Composition Root & Core Simplification
- [ ] Clean up deprecated types or aliases in `core/composer.go` once service signatures stabilize.
- [ ] Run full test suites (`task envx:test` or `task test`) and lint checks.
