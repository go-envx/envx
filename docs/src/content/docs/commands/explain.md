---
title: explain
description: Show each resolved value and the file it came from.
sidebar:
  order: 6
---

```sh
envx explain <project> [key]
```

`explain` resolves a project's environment and reports, for each key, the value
and the file it was resolved from. With no key it explains every key; with a key
it explains just that one.

Values are masked by default. Pass `--reveal` to print them in plaintext, or
`--output json` for machine-readable output.

## Arguments

| Argument | Description |
| --- | --- |
| `<project>` | The project to resolve. |
| `[key]` | Optional. Explain only this key instead of all keys. |

## Examples

These examples use the [Example Workspace](/guide/example-workspace/) (`envx create example-workspace`).

```sh
# Explain every key for the default environment.
envx explain api-service

# Explain a single key and reveal its value.
envx explain api-service DATABASE_HOST --reveal

# Emit machine-readable output.
envx explain api-service --output json
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--reveal` | | Print values in plaintext instead of masking them. |
| `--output`, `-o` | | Output format: `table` (default) or `json`. |
| `--env`, `-E` | `ENVX_ENV` | Target environment to resolve. |
| `--require-overlays` | `ENVX_REQUIRE_OVERLAYS` | Require every overlay file to exist. |
| `--prefix` | `ENVX_PREFIX` | Prefix prepended to every key. |
| `--suffix` | `ENVX_SUFFIX` | Suffix appended to every key. |
| `--delimiter` | `ENVX_DELIMITER` | String used to join list values. |
| `--namespace-prefix` | `ENVX_NAMESPACE_PREFIX` | Prefix each key with its namespace. |

Plus the [global flags](/commands/overview/#global-flags). The `--env`,
`--require-overlays`, `--prefix`, `--suffix`, `--delimiter`, and `--namespace-prefix` flags
map to [settings](/configuration/schema/#settings-2) and override the values in
`envx.yaml`.
