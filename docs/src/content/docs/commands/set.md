---
title: set
description: Write a key/value into a namespace's environment overlay file.
sidebar:
  order: 4
---

```sh
envx set <include-path> <key> <value>
```

`set` writes a key/value pair into the environment overlay file for a namespace.
The key supports dot notation for nested YAML paths, such as
`credentials.password`.

The include path must match an entry from a project's `includes` list exactly,
for example `env/postgres` or `apps/api-core/env/api-core`. The value is written
to the overlay for the target environment (`<include-path>.<env>.yaml`).

## Arguments

| Argument | Description |
| --- | --- |
| `<include-path>` | A namespace path exactly as it appears in a project's `includes`. |
| `<key>` | The key to write; dot notation sets a nested path. |
| `<value>` | The value to write. |

## Examples

```sh
# Write a top-level key to the default environment's overlay.
envx set env/postgres password insecure-password

# Write a nested key to the staging overlay.
envx set env/postgres credentials.password s3cret --env staging

# Write to the production overlay.
envx set env/gateway timeout 10 --env production
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--env`, `-E` | `ENVX_ENV` | Environment overlay file to write to. |

Plus the [global flags](/commands/overview/#global-flags).
