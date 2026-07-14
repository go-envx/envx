---
title: get
description: Resolve a project's environment and print a single value.
sidebar:
  order: 2
---

```sh
envx get <project> <key>
```

`get` resolves the merged environment for a project and prints the value of a
single key. The key is matched case-insensitively, so `HOST` and `host` resolve
to the same value.

## Arguments

| Argument | Description |
| --- | --- |
| `<project>` | The project to resolve, as declared in `envx.yaml`. |
| `<key>` | The key to print (case-insensitive). |

## Examples

```sh
# Print a value for the default environment.
envx get api-core POSTGRES_HOST

# Resolve against a specific environment.
envx get api-core postgres_host --env production

# Prefix keys before looking one up.
envx get api-core APP_POSTGRES_HOST --prefix APP
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--env`, `-E` | `ENVX_ENV` | Target environment to resolve. |
| `--strict` | `ENVX_STRICT` | Require every overlay file to exist. |
| `--prefix` | `ENVX_PREFIX` | Prefix prepended to every key. |
| `--suffix` | `ENVX_SUFFIX` | Suffix appended to every key. |
| `--delimiter` | `ENVX_DELIMITER` | String used to join list values. |
| `--namespace-prefix` | `ENVX_NAMESPACE_PREFIX` | Prefix each key with its namespace. |

Plus the [global flags](/commands/overview/#global-flags). These flags map to
[settings](/configuration/settings-reference/) and override the values in `envx.yaml`.
