# envx Secrets Design

This document defines the target architecture for encrypted secrets in envx and the sequence of changes used to reach it. The package stubs describe intended contracts; the Tasks section records what is already present on `main` and what remains.

## Overview

### Status

`main` resolves explicit `secret://group/key` references from a workspace `secrets.yaml` file. It also contains algorithm-neutral cipher implementations in a top-level `internal/cipher` package, the top-level `internal/privatekey` boundary, and the secrets feature's nested `envelope` and `store` implementation packages. The store still permits plaintext migration entries, and the top-level `envx keypair generate|inspect|print` tree is registered as the first management command surface.

### Target behavior

- `secrets.yaml` is committed workspace state containing public keys and encrypted values; private keys never belong in it.
- A reference has the explicit form `secret://<group>/<key>`. A leading backslash escapes a literal reference, and implicit environment-based groups are not supported.
- A group is a trust boundary with one public key and any number of encrypted values. Group names commonly match environments, but they are an independent namespace and may include groups such as `shared`.
- Automatic private-key lookup is deterministic: `ENVX_PRIVATE_KEY_<GROUP>`, then the combined `ENVX_PRIVATE_KEY`, then the local file at `secrets.keys_path`.
- Automatic lookup never prompts. A no-echo prompt or stdin is an explicit source selected for one operation, so scripts and `envx run` never block unexpectedly.
- Commands that promise plaintext, including `run`, revealing `secrets get`, `secrets decrypt`, and `secrets rotate`, fail when a required key is unavailable. `keypair inspect` instead reports `not_available` as a valid status.
- Private keys are transient capabilities. They are never included in status objects, mutation reports, logs, errors, or normal command output.
- Secret mutation is exposed through an immutable `Manager` whose intent-level methods own validation, read-modify-write behavior, and cross-file ordering. CLI actions remain thin adapters.
- Dangling references fail loudly. Diagnostic commands may later render a dangling state instead of aborting, but they must never pass an unresolved reference to a child process as plaintext.

The target store shape is:

```yaml
public_keys:
	production: age-public-key:age1...
	shared: age-public-key:age1...

secrets:
  production:
    database_password: encrypted-age:...
  shared:
    service_token: encrypted-age:...
```

Ciphertext uses the existing `encrypted-{algorithm}:{base64url-payload}` envelope. The envelope selects the decrypting implementation and allows supported algorithms to coexist without a workspace-wide cipher switch.

### Security policy

Encryption protects committed data; it does not authorize an operation. Possession of a private key enables decryption, so plaintext is materialized only for a command whose contract requires it. `secrets set` accepts plaintext through an explicit secure input path, while bulk encryption may temporarily accept plaintext values in `secrets.yaml` for migration convenience and validation must flag any value left unencrypted.

Each mutating operation validates its complete input and computes its changes before writing. Individual files are replaced atomically, private-key files are always `0600`, and normal files preserve their existing mode or default to `0644`. A two-file keypair operation cannot be fully transactional: it writes or hands off the private key before committing matching public state and returns an actionable recovery error if the second write fails.

### Future constraints

`validate` will inspect every project and environment for dangling references, orphaned values, plaintext store entries, invalid keypairs, and unavailable keys. `emit` will either materialize plaintext for targets such as dotenv and Kubernetes Secrets or preserve provider references for compatible targets. External key providers will implement the same private-key source and destination ports; when the first provider lands, configuration may add a distinct ordered `secrets.key_sources` list. The v1 `keys_path` field remains one local filesystem path and is never overloaded with provider URIs.

### Resolved policy

- No group requires additional confirmation or authorization to materialize commands; key availability remains a capability, not an intent signal.
- Group identity is case-insensitive everywhere, including `secrets.yaml` storage and reference matching. Environment-variable addressing continues to normalize groups to uppercase.

## Architecture

The stubs below are the intended contracts. Existing symbols keep their current shape where practical; additions are implemented only in the task that needs them.

### `internal/cipher`

`cipher` owns algorithms and opaque key formats. It does not read files, environment variables, secret references, or terminal input.

```go
// Algorithm identifies a supported ciphertext algorithm.
type Algorithm string

// Keypair contains transient public and private key material.
type Keypair struct {
	PublicKey  string
	PrivateKey string
}

// Cipher performs algorithm-specific key and value operations.
type Cipher interface {
	Algorithm() Algorithm
	Keypair() (Keypair, error)
	ValidateKeypair(publicKey, privateKey string) error
	Encrypt(plaintext, publicKey string) ([]byte, error)
	Decrypt(ciphertext []byte, privateKey string) (string, error)
}

// Params selects a cipher implementation and supplies its algorithm-specific options.
type Params struct {
	Algorithm Algorithm
	Options   AlgorithmOptions
}

// New constructs the cipher implementation selected by params.
func New(params Params) (Cipher, error)
```

Age is the default algorithm and NaCl sealed boxes are also implemented. `ValidateKeypair` validates key format and public/private correspondence without exposing private material, backing inspection and rotation.

The secrets feature keeps representation-specific implementation packages beneath the root `internal/secrets` boundary. Cipher and private-key material live in the sibling top-level `internal/cipher` and `internal/privatekey` packages rather than under `internal/secrets`. The root package's own file layout is described in `internal/secrets` below.

```text
internal/cipher/
internal/privatekey/
internal/secrets/
	internal/
		envelope/
		store/
```

### `internal/secrets/internal/envelope`

`envelope` owns the algorithm-tagged ciphertext format used in `secrets.yaml`. It encodes and decodes `encrypted-{algorithm}:{base64url-payload}` values and validates their grammar without reading files or selecting a cipher implementation.

### `internal/privatekey`

`privatekey` owns key-resolution and key-destination mechanisms plus lookup provenance. It is a top-level sibling of the secrets boundary and the `envx.keys` counterpart to the store: it handles transient private-key material and local key-file persistence, but does not generate keypairs, inspect public keys, edit `secrets.yaml`, choose command policy, or prompt automatically.

```go
// PrivateKey holds transient material and its provenance.
type PrivateKey struct {
	Value  string
	Origin string
}

// ErrNotAvailable indicates that no source has a key for the group.
var ErrNotAvailable error

// Resolver resolves one group's private key.
type Resolver interface {
	Resolve(group string) (PrivateKey, error)
}

// Destination receives newly generated private-key material.
type Destination interface {
	Write(group, privateKey string) error
}

// ResolverOptions configures automatic environment-then-file lookup.
type ResolverOptions struct {
	KeysPath  string
	LookupEnv func(string) (string, bool)
}

// NewResolver creates the automatic environment-then-file resolver.
func NewResolver(options ResolverOptions) Resolver

// NewFileDestination creates a private NAME=value key-file destination.
func NewFileDestination(path string) Destination

// NewWriterDestination performs an explicit one-operation handoff.
func NewWriterDestination(writer io.Writer) Destination

// FilePath reports a destination's key-file path when it exposes one.
func FilePath(destination Destination) (string, bool)
```

Only `ErrNotAvailable` means another source may be attempted. An explicitly present empty value, malformed combined environment variable, unreadable file, or duplicate group stops resolution with an error. Environment variables are read-only sources and never implicit destinations.

### `internal/secrets/internal/store`

`store` is the target document boundary for `secrets.yaml`. It preserves YAML structure where practical, validates the complete document, and atomically persists public keys and stored values. It never reads private keys, decrypts values, or implements command workflows.

```go
// Secret identifies one stored value, which may be plaintext or ciphertext.
type Secret struct { Group, Key, Value string }

// Document is the validated mutable representation bound to the path passed to Open.
type Document struct {
	path string
	// unexported document state
}

// Open reads a missing or existing secrets document and binds its path.
func Open(path string) (*Document, error)

// PublicKey returns a group's public key and whether it exists.
func (d *Document) PublicKey(group string) (string, bool)

// Secret returns one stored secret and whether it exists.
func (d *Document) Secret(group, key string) (Secret, bool)

// Secrets returns stored secrets in document order.
func (d *Document) Secrets() []Secret

// SetPublicKey updates one group's public key in memory.
func (d *Document) SetPublicKey(group, publicKey string) error

// SetSecret updates one stored value in memory.
func (d *Document) SetSecret(group, key, value string) error

// DeleteSecret removes one stored value in memory.
func (d *Document) DeleteSecret(group, key string) (bool, error)

// Save atomically persists this validated document to its bound path.
func (d *Document) Save() error
```

The package is internal machinery: actions and unrelated features use the root `secrets` contract rather than mutate a `Document`.

### `internal/secrets`

The root package is the feature boundary. It combines the store, private-key access, and cipher implementations; owns cross-resource invariants; and exposes an immutable `Manager` plus a read resolver. `Manager` binds workspace paths and injected dependencies once, so operation call sites do not repeat ambiguous `Settings` and private-key option arguments. It does not cache a mutable store document: each method opens, validates, performs its workflow, and persists atomically.

The package stays a single Go package organized by concept, one file per cohesive operation group rather than one file per method. Files are named by verb or concept with no `manager_` prefix, since every operation attaches to `Manager` and the prefix would only restate the package. `manager.go` holds construction only (`Params`, `Manager`, `New`); `types.go` holds result DTOs. When a method grows real algorithm or I/O weight, that logic moves into an `internal/` subpackage instead of enlarging the method file.

```text
internal/secrets/
	manager.go    # Params + Manager + New
	types.go      # result DTOs
	reference.go  # secret:// grammar
	resolver.go   # Resolver() + Resolve
	validate.go   # name validation
	keypair.go    # GenerateKeypair, InspectKeypair, RotateKeypair
	secret.go     # Set, Get, Has, Delete (single-secret CRUD)
	encrypt.go    # Encrypt, Decrypt (bulk)
	internal/
		envelope/
		store/
```

```go
// Params supplies workspace paths and dependencies once at construction.
type Params struct {
	// SecretsPath is the secrets.yaml store location.
	SecretsPath string
	// KeysPath is the local private-key file location.
	KeysPath string
	// DefaultIndent is the block indentation applied when the store has none of
	// its own; the configuration layer supplies it.
	DefaultIndent int
	// Cipher performs key generation and encryption operations.
	Cipher cipher.Cipher
	// PrivateKeyResolver resolves private-key material for read operations.
	PrivateKeyResolver privatekey.Resolver
	// PrivateKeyDestination receives newly generated private-key material.
	PrivateKeyDestination privatekey.Destination
}

// Manager coordinates secret workflows without exposing its mutable state.
type Manager struct {
	// unexported paths and dependencies
}

// New binds workspace paths and dependencies into a manager.
func New(params Params) (*Manager, error)

// ResolverParams controls reference materialization for one resolver.
type ResolverParams struct {
	Reveal bool
}

// Resolver creates a value resolver with the requested materialization policy.
func (m *Manager) Resolver(params ResolverParams) (*Resolver, error)

// PrivateKeyStatus reports safe key availability and validity.
type PrivateKeyStatus string

// KeypairMetadata reports public metadata without private-key material.
type KeypairMetadata struct {
	Group            string
	PublicKey        string
	PrivateKeyStatus PrivateKeyStatus
}

// SecretReference identifies one secret by group and key without carrying its value.
// It is used for identity-bearing results and internal reference handling; exact
// Manager operations accept group and key arguments directly.
type SecretReference struct { Group, Key string }

// UpdateResult reports changed keypairs and secret references.
type UpdateResult struct {
	Keypairs []KeypairMetadata
	Secrets  []SecretReference
}

// Resolve passes through plain values and resolves or masks secret references.
func (r *Resolver) Resolve(value, environment string) (string, error)

// Get decrypts and returns one value.
func (m *Manager) Get(group, key string) (string, error)

// PlaintextResolver lazily supplies one secret plaintext value.
type PlaintextResolver func() (string, error)

// Set lazily obtains, encrypts, and stores one plaintext value.
func (m *Manager) Set(group, key string, plaintext PlaintextResolver) error

// Has reports presence without loading a private key.
func (m *Manager) Has(group, key string) (bool, error)

// Delete removes one value without tearing down its group identity.
func (m *Manager) Delete(group, key string) error

// GenerateKeypair creates a missing identity.
func (m *Manager) GenerateKeypair(group string) (KeypairMetadata, error)

// InspectKeypair reports not_available, valid, or invalid without writing or prompting.
func (m *Manager) InspectKeypair(group string) (KeypairMetadata, error)

// RotateKeypair replaces an identity and re-encrypts its complete group.
func (m *Manager) RotateKeypair(group string) (UpdateResult, error)

// Encrypt encrypts matching plaintext store entries in place. Empty group or key
// matches all values in that dimension.
func (m *Manager) Encrypt(group, key string) (UpdateResult, error)

// Decrypt decrypts matching ciphertext store entries in place. Empty group or key
// matches all values in that dimension.
func (m *Manager) Decrypt(group, key string) (UpdateResult, error)

```

The composition layer supplies concrete cipher, resolver, and destination dependencies; `New` rejects missing paths or dependencies. Automatic reads use environment-then-file lookup. New key material defaults to the configured file destination for ordinary local generation, but rotation rejects that implicit destination when provenance shows the active old key came from an environment variable or explicit one-operation source. Exact operations require non-empty group and key arguments; `SecretReference` remains the value object for identity-bearing results and internal reference handling. Bulk encryption and decryption use inline empty-string filters rather than overloading exact-identity arguments with wildcard semantics.

Typical action wiring constructs the manager once, then passes only operation data at each call site:

```go
manager, err := secrets.New(secrets.Params{ /* paths and dependencies */ })
resolver, err := manager.Resolver(secrets.ResolverParams{Reveal: reveal})
value, err := manager.Get(group, key)
result, err := manager.Encrypt(group, key)
```

### `internal/schema` and `internal/config`

Schema owns the workspace manifest shape, while config resolves paths and composes dependencies for a command. Secret paths are workspace-level and not project-overridable.

```go
// SecretsConfig configures workspace secret state.
type SecretsConfig struct {
	SecretsPath string `yaml:"path"`
	KeysPath    string `yaml:"keys_path"`
}

// ResolveProject resolves a project and wires its secret resolver.
func ResolveProject(input *Input, project string, reveal bool) (*Result, error)

// ResolveWorkspace resolves paths for management commands without building a project.
func ResolveWorkspace(input *Input) (*Result, error)
```

An empty `path` defaults to `secrets.yaml` beside `envx.yaml`. An empty `keys_path` defaults to `envx.keys` beside the resolved secrets file; an explicit relative value is resolved from the manifest directory. Config builds `Params` and chooses resolver policy, but does not prompt or implement secret workflows.

### `internal/actions/keypair` and `internal/actions/secrets`

Actions are limited to Cobra concerns: arguments, flags, terminal capability, explicit input selection, calls into `secrets`, and safe rendering. Keypair management is a top-level command tree: `envx keypair generate`, `envx keypair inspect`, `envx keypair rotate`, and `envx keypair print` (an ephemeral pair written only to stdout). The `envx secrets` tree currently provides `set`, `get`, `encrypt`, `decrypt`, and `delete`. Prompt and stdin adapters are constructed only when explicitly requested; normal automatic execution never reads the terminal.

`envx run` remains outside this command tree and always requests revealed resolution because a child process needs plaintext. It completes all resolution before starting the process, so a missing or invalid private key cannot produce a partially configured child.

## Tasks

Each item is one logical PR based on the preceding item after it merges to `main`. A task includes focused tests and leaves no live panic or unreachable placeholder API.

1. ✅ **Secret references:** Resolve explicit `secret://group/key` values after merge winners are selected, preserve escaped literals, and fail on dangling references.
2. ✅ **Cipher foundation:** Provide age and NaCl-box implementations behind `cipher.Cipher` plus the algorithm-tagged ciphertext envelope and composition tests.
3. ✅ **Private-key contract:** Define provenance-aware environment/file sources, file/writer destinations, fail-closed parsing, and private atomic file writes; keep the capability intentionally unwired from command workflows.
4. ✅ **Keypair validation:** Implement `ValidateKeypair` for the cipher backends and add focused validation tests.
5. ✅ **Key-path configuration:** Add scalar `secrets.keys_path`, default it beside the resolved store, expose it through `secrets.Params`, and test default, relative, and absolute paths.
6. ✅ **Store document boundary:** Extract `internal/secrets/internal/store`, add `public_keys` and ciphertext-aware validation, preserve comments and order during mutation, and keep its API inaccessible to actions.
7. ✅ **Private-key package:** Create `internal/privatekey` as the top-level `envx.keys` implementation boundary. Provide provenance-aware environment/file resolvers, file/writer destinations, fail-closed parsing, private atomic writes, and focused tests without coupling the package to store mutation, keypair workflows, or CLI policy.
8. ✅ **Keypair generation and inspection:** Implemented `GenerateKeypair` and `InspectKeypair` in the root manager with Git-ignore and write-order safety, private-material-free metadata, and thin top-level `envx keypair generate|inspect|print` adapters (`print` emits an ephemeral pair to stdout without touching workspace files).
9. ✅ **Secure single-secret entry:** Implement `Manager.Set` and `envx secrets set` with explicit stdin or no-echo terminal input, up-front validation, and no plaintext argument or output.
10. ✅ **Read and presence verbs:** Implement `Manager.Get` and `Manager.Has`, then expose `envx secrets get` with plaintext available only through its deliberate reveal contract.
11. ✅ **Deletion:** Implemented one-entry `Manager.Delete` (validate, remove, atomic save; missing entry is a dangling-reference error) and the thin `envx secrets delete <group> <key>` adapter. Deletion never removes a group's public key or its remaining values; group teardown remains deferred until its retention semantics are designed.
12. ✅ **Runtime reveal:** Implemented decrypt-on-lookup in the secrets resolver with a mask/reveal policy: `Manager.Resolver(ResolverParams{Reveal})` masks references to their canonical `secret://group/key` form by default (no private key touched) and decrypts on reveal, resolving each group's private key lazily and caching it. `config.ResolveProject(in, project, reveal)` threads the policy; `envx run` always reveals so a decryption failure aborts during resolution before the child starts, while `get`/`explain`/`diff` mask by default and reveal only with `--reveal`. This is the local-secrets MVP.
13. ✅ **Bulk encryption and decryption:** Implemented selector-scoped `Manager.Encrypt` and `Manager.Decrypt`, where an empty group or key widens the selection to every value in that dimension. Both stage all changes in memory and persist atomically only after every match succeeds, so a mid-operation failure leaves the store untouched. Encrypt skips values already carrying a ciphertext envelope and Decrypt skips plaintext, making each idempotent; Decrypt resolves each group's private key lazily and caches it. An explicit selector matching no stored entry is an error, and both return an `UpdateResult` reporting changed identities without their values. Thin `envx secrets encrypt` and `envx secrets decrypt` adapters expose the operations with `--group/-g` and `--key/-k` selectors, reporting only changed identities.
14. ✅ **Keypair rotation:** Implemented `Manager.RotateKeypair` and registered `envx keypair rotate`. It requires the group's current private key, decrypts the entire group in memory, generates and validates a replacement identity, and re-encrypts every value under the new public key. The new private key is delivered before the new public state is committed, mirroring generation's safe write order. Destination provenance is enforced: rotation refuses the implicit local key file when the active key came from a higher-priority environment or explicit source that would shadow the new key. A file-backed private key is preserved as rollback material until the store commit succeeds, and a failed commit returns an actionable recovery error that points at the preserved key without exposing private material.
15. ⬜ **Workspace validation:** Aggregate all projects and environments, report dangling and orphaned entries, plaintext values, unavailable keys, and invalid keypairs, with strict mode escalating warnings.
16. ⬜ **Emit:** Add resolve-and-render targets with an explicit choice between plaintext materialization and reference forwarding.
17. ⬜ **External providers:** Introduce provider adapters and a distinct ordered `secrets.key_sources` field only with the first working provider; retain environment variables as the fixed highest-priority automatic sources unless an operation explicitly overrides the source.
