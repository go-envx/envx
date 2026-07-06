# envx — Secrets & Emit Design Plan

Status: **approved direction, not yet implemented**. This document captures the
agreed architecture for two roadmap features and the actions that support them.
Locations/paths here are proposals; adjust freely.

## Guiding principles

- **Config stays clean; secrets are references.** Namespace `.yaml` files remain
  human-readable config. A secret value is a *reference* the merge pipeline
  dereferences, never inline ciphertext.
- **One resolver seam, many backends.** A reference has a scheme; each scheme has
  a resolver. The first backend is a local encrypted store (`secret://`); external
  managers (`gcpsm://`, `aws://`, `doppler://`) are future resolvers behind the
  same interface.
- **envx owns resolution, location, secrets, and emission — not free-form config
  editing.** Config data-entry is best done by a human or an AI agent editing YAML
  directly, aided by `explain --output=json` and a locate helper. Secret entry is
  a structured, encrypted operation that never exposes plaintext to an agent.

## 1. Secret references

- **Syntax:** scheme-prefixed string value, e.g.
  `password: secret://production/postgres_password`.
  - `secret://` = local encrypted store (this plan).
  - Future: `gcpsm://…`, `aws://…`, `doppler://…`.
- **Identity is `group/key`.** The group selects the encryption key (see §2); the
  key is the secret's logical name. Keys may not contain `/`.
- **Grammar & implicit-group sugar:**
  - `secret://group/key` (one `/`) — explicit group.
  - `secret://key` (no `/`) — the group **defaults to the active environment**,
    so a single reference in a base file resolves per-env (base `postgres.yaml`
    with `secret://postgres_password` resolves to `development/postgres_password`,
    `production/postgres_password`, …). Use `secret://shared/…` for the cross-env
    exception.
  - The sugar is on by default and can be **disabled** via the workspace secrets
    config key `require_group: true`; when disabled, a single-segment reference
    is an error ("has no group and the shorthand is disabled").
- **Escape hatch (literal values):** a leading backslash escapes a value that
  legitimately looks like a reference — `\secret://x` resolves to the literal
  `secret://x`. envx strips one leading `\` before scheme detection. Because `\`
  is an escape inside YAML *double* quotes, document using an unquoted or
  single-quoted scalar (`'\secret://x'`). This is a rare safety valve.
- **Resolution:** a per-leaf dereference applied during flattening. Each
  flattened leaf value (a scalar, and every item of a list before it is joined)
  is passed through the resolver exactly once; if it matches a known `scheme://`
  it is dereferenced, otherwise it passes through. Running at the leaf means a
  reference works anywhere a value does, including inside a list. Reuses the
  existing masking gate (`--reveal`) to decide when plaintext is materialized.
- **Composition with overlays:** an env-specific reference lives in that env's
  overlay (only seen when that env resolves); a `shared` reference in a base file
  applies to every environment.

## 2. Secrets store (`secrets.yaml`)

- **Separate file, alongside `envx.yaml`.** Auto-discovered beside the manifest
  via the existing walk-up discovery; configurable via the `secrets:` block
  (see below). Kept separate for lifecycle, review, and blast-radius reasons;
  tooling (`encrypt`, `validate`, `explain`) means users rarely hand-edit it.
- **Config lives in a workspace-level `secrets:` block** (a top-level manifest
  key, *not* under `settings:` and *not* project-overridable — secrets are a
  workspace concern, and keeping them out of per-project settings preserves the
  centralized-store model). Both fields are optional; the toggle defaults false
  (shorthand on):

  ```yaml
  secrets:
    path: ./secrets.yaml     # default: secrets.yaml beside envx.yaml
    require_group: false     # default false; true = require secret://<group>/<key>
  ```
- **Shape** (nested under `secrets:` so groups can't collide with `public_keys`):

  ```yaml
  public_keys:
    production: age1...
    staging: age1...
    development: age1...
    shared: age1...

  secrets:
    production:
      postgres_password: <ciphertext>
    staging:
      postgres_password: <ciphertext>
    development:
      postgres_password: <ciphertext>
    shared:
      claude_api_key: <ciphertext>
  ```

- **Named keypairs (key-groups).** A group has one public key (in `public_keys`)
  and holds the ciphertexts under it. **A key-group is a trust boundary.**
  - **Convention:** one group per declared environment, plus occasional `shared`
    groups for genuinely cross-environment, comparable-sensitivity secrets.
  - Use `shared` sparingly; never lump high-sensitivity secrets (e.g. a prod DB
    password) into a broad shared group.
  - Group names are an independent namespace; environments are the recommended
    default set of group names, not a hard requirement.

## 3. Keys & crypto

- **Asymmetric encryption.** Public key encrypts (add/rotate a secret with only
  the public key — great for developers and CI); private key decrypts.
- **Public keys** live in `secrets.yaml` (`public_keys`), committed.
- **Private keys** live in a git-ignored `envx.keys` file using `NAME=priv` lines:

  ```
  DEVELOPMENT=priv_...
  STAGING=priv_...
  PRODUCTION=priv_...
  SHARED=priv_...
  ```

- **Key sources & precedence** (env over file, most-specific first), all sharing
  the same `NAME=priv` format:
  1. `ENVX_PRIVATE_KEY_<GROUP>` — per-group env var (most specific; CI-friendly).
  2. `ENVX_PRIVATE_KEY` — combined `NAME=priv` lines in one var.
  3. `envx.keys` file — local dev, git-ignored.

  This mirrors dotenvx (`.env.keys` for local, `DOTENV_PRIVATE_KEY*` env vars win
  in CI/prod) and envx's own `flag > ENVX_* > file` precedence.
- **Key availability at resolve/emit:** envx needs the private key for every
  group referenced by the active resolution — typically the active environment's
  group plus any referenced `shared` groups. Missing key → hard error.
- **Cipher seam (v1) + selectable cipher (v2).** Route all crypto through a small
  `Cipher` interface (`Keypair` / `Encrypt` / `Decrypt`) so the primitive is
  swappable. **v1 ships age (X25519, `filippo.io/age`) as the only implementation**
  — the default. **v2** exposes selection via a top-level `cipher: age` field in
  `secrets.yaml` plus a name→impl registry; tag each ciphertext with a short
  algorithm marker so ciphers can coexist during migration.

## 4. New actions

- `keypair` — generate a group's keypair; write the public key into
  `secrets.yaml` `public_keys`, the private key into `envx.keys`.
- `encrypt` — add/update a secret. Plaintext read from **stdin or a prompt**
  (never an arg, never an AI agent). Encrypts with the group's public key; writes
  a structured store entry. No namespace-file editing.
- `decrypt` — materialize a secret's plaintext (debugging / piping), gated by
  private-key availability.
- `validate` — see §5.
- `emit` — see §6.

### `set` repositioning

- Do **not** grow `set` into a general free-form config editor.
- **Config entry** (non-secret): human/AI-agent editing the YAML directly,
  powered by `explain --output=json` plus a small **locate** helper ("for this
  project+key, which file and nested path backs the resolved value?"). envx
  provides the map; it does not reimplement a YAML editor.
- **Secret entry:** the `encrypt` action (structured store, no agent plaintext).
- Keep the existing guarded `set` narrow (scripted/CI config writes) or retire it
  for config; do not invest in robustifying free-form YAML editing.

## 5. Validation (`validate` command)

Whole-workspace operation: aggregates references across **every project × every
declared environment** (a reference or orphan is only meaningful workspace-wide).

- **Dangling reference** (`secret://g/k` with no matching `g`/`k`) → **error**;
  also fails any normal resolve (`get`/`run`/`explain`/`emit`), not just
  `validate`.
- **Orphan secret** (in the store, referenced nowhere) → **warning** (non-fatal
  by default).
- **Missing decryption key** for a referenced group → **error** (at resolve/emit;
  `validate` reports it as a warning/error per `--strict`).
- **`--strict`** escalates warnings to errors (CI). Exit non-zero on any error.
- Natural home for future lints (missing include files, flatten collisions, etc.).

## 6. Emit (`emit` command)

- A "resolve → render" action, like `get`/`explain`/`diff`.
- **Formats:** `.env`, flattened `.yaml`, k8s ConfigMap, k8s Secret, JSON; later,
  push to external providers (GCP/AWS/Doppler).
- **Secrets handling per target:** either **materialize plaintext** (decrypt;
  gated by key availability; e.g. `.env`, k8s `Secret`) or **forward the
  reference** (e.g. a k8s manifest pointing at an external manager via the
  External Secrets Operator). Reuses the `--reveal`/masking gate.

## 7. External backends (future)

Additional reference schemes (`gcpsm://`, `aws://`, `doppler://`) implemented as
resolvers behind the same interface as `secret://`. `emit` chooses to materialize
or forward per target.

## Suggested phasing

1. **Reference seam** — ✅ **done**. `internal/secrets` (`Store` + `Resolver`),
   the `envmerge.Resolver` interface + per-leaf dereference during flattening,
   the workspace `secrets:` block, and config wiring. Ships with a plaintext
   store (no crypto yet) proving the plumbing: implicit/explicit/shared groups,
   references inside lists, URL passthrough, the backslash escape, `explain`
   masking, and dangling-reference errors.
2. **Local crypto store** — `keypair`, `encrypt`, `decrypt`; ciphertext in the
   store; `envx.keys`; wire the `secret://` resolver to decrypt.
3. **`validate`** — bidirectional checks + `--strict`.
4. **`emit`** — output formats, materialize-vs-forward gate.
5. **External backends** — additional resolver schemes.

`set` repositioning (`explain --output=json` + locate helper) can proceed
alongside phase 1–2.

## Open decisions to confirm before/during build

- **Resolved:** cipher = age default behind a `Cipher` seam in v1, user-selectable
  in v2 (§3). Escape hatch = leading backslash (§1). Implicit-group sugar =
  included now, toggle `secrets.implicit_group` (§1–2). Key precedence =
  `ENVX_PRIVATE_KEY_<GROUP>` > `ENVX_PRIVATE_KEY` > `envx.keys` (§3).
- Exact `NAME` casing/normalization for key vars vs group names
  (e.g. `ENVX_PRIVATE_KEY_PRODUCTION` ↔ group `production`).
- age identity/recipient string handling and armored ciphertext encoding in YAML.
