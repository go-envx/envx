---
title: Configuration
description: Settings, sources, and the resolution precedence.
sidebar:
  order: 2
---

Settings can be provided as CLI flags, `ENVX_*` environment variables, or in the
manifest (per project or globally). They resolve with this **precedence**, highest
to lowest:

1. **CLI flag** (e.g. `--env production`)
2. **`ENVX_*` environment variable** (e.g. `ENVX_ENV=production`)
3. **Manifest project settings** (`projects.<name>.settings`)
4. **Manifest global settings** (top-level `settings`)
5. **Default value**

:::note
This precedence governs how *settings* are resolved. The environment *values*
themselves are resolved by namespace merge order (last wins). See
[Concepts](/guide/concepts/).
:::

| Setting | Flag | Env var | Manifest key | Default |
| --- | --- | --- | --- | --- |
| Manifest path | `--config` | `ENVX_CONFIG` | none | auto-discovered |
| Target environment | `--env`, `-E` | `ENVX_ENV` | `env` | first declared environment |
| Require overlay files | `--strict` | `ENVX_STRICT` | `strict` | `false` |
| Key prefix | `--prefix` | `ENVX_PREFIX` | `prefix` | `""` |
| Key suffix | `--suffix` | `ENVX_SUFFIX` | `suffix` | `""` |
| List join delimiter | `--delimiter` | `ENVX_DELIMITER` | `delimiter` | `","` |
| Prefix keys with namespace | `--namespace-prefix` | `ENVX_NAMESPACE_PREFIX` | `namespace_prefix` | `false` |
| File values override OS env | `--overload` | `ENVX_OVERLOAD` | `overload` | `false` |

Two presentation-only flags apply to `explain` and `diff`:

| Flag | Description | Default |
| --- | --- | --- |
| `--reveal` | Print values in plaintext instead of masking them | masked |
| `--output`, `-o` | Output format: `table` or `json` | `table` |

## Per-project overrides

Settings can be applied globally and overridden per project:

```yaml
settings:
  strict: true          # applies to every project

projects:
  api:
    includes: [env/api]
    settings:
      prefix: API        # overrides the global setting for this project only
```
