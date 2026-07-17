---
title: get
description: Resolve a project's environment and print a single value.
sidebar:
  order: 3
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

These examples use the [Example Workspace](/guide/example-workspace/) (`envx create example-workspace`).

```sh
# Print a value for the default environment.
envx get api-service DATABASE_HOST

# Resolve against a specific environment.
envx get api-service database_host --env production
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--env`, `-E` | `ENVX_ENV` | Target environment to resolve. |
| `--require-overlays` | `ENVX_REQUIRE_OVERLAYS` | Require every overlay file to exist. |
| `--prefix` | `ENVX_PREFIX` | Prefix prepended to every key. |
| `--suffix` | `ENVX_SUFFIX` | Suffix appended to every key. |
| `--delimiter` | `ENVX_DELIMITER` | String used to join list values. |
| `--namespace-prefix` | `ENVX_NAMESPACE_PREFIX` | Prefix each key with its namespace. |

Plus the [global flags](/commands/overview/#global-flags). These flags map to
[settings](/configuration/schema/#settings-2) and override the values in `envx.yaml`.
