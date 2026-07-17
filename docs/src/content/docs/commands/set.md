---
title: set
description: Write a key/value into a namespace's environment overlay file.
sidebar:
  order: 5
---

```sh
envx set <include-path> <key> <value>
```

`set` writes a key/value pair into the environment overlay file for a namespace.
The key supports dot notation for nested YAML paths, such as
`credentials.password`.

The include path must match an entry from a project's `includes` list exactly,
for example `env/database` or `api-service/env/values`. The value is written
to the overlay for the target environment (`<include-path>.<env>.yaml`).

## Arguments

| Argument | Description |
| --- | --- |
| `<include-path>` | A namespace path exactly as it appears in a project's `includes`. |
| `<key>` | The key to write; dot notation sets a nested path. |
| `<value>` | The value to write. |

## Examples

These examples use the [Example Workspace](/guide/example-workspace/) (`envx create example-workspace`).

```sh
# Write a flat key to an app's overlay.
envx set api-service/env/values log_level warn --env production

# Write a nested key with dot notation.
envx set env/database database.password rotated --env production

# Write to the default environment's overlay.
envx set env/gateway gateway.timeout 10
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--env`, `-E` | `ENVX_ENV` | Environment overlay file to write to. |

Plus the [global flags](/commands/overview/#global-flags).
