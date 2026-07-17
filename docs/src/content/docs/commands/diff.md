---
title: diff
description: Compare a project's resolved environment across two environments.
sidebar:
  order: 7
---

```sh
envx diff <project> <env-a> <env-b>
```

`diff` resolves the same project under two environments and reports the
differences: keys added, removed, or changed between `env-a` and `env-b`.

Values are masked by default. Pass `--reveal` to print them in plaintext, or
`--output json` for machine-readable output.

## Arguments

| Argument | Description |
| --- | --- |
| `<project>` | The project to resolve under both environments. |
| `<env-a>` | The first environment to compare. |
| `<env-b>` | The second environment to compare. |

Because the environments are given as arguments, `diff` does not take an `--env`
flag.

## Examples

These examples use the [Example Workspace](/guide/example-workspace/) (`envx create example-workspace`).

```sh
# Compare development against production.
envx diff api-service development production

# Reveal the differing values.
envx diff api-service development production --reveal

# Emit machine-readable output.
envx diff api-service development production --output json
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--reveal` | | Print values in plaintext instead of masking them. |
| `--output`, `-o` | | Output format: `table` (default) or `json`. |
| `--require-overlays` | `ENVX_REQUIRE_OVERLAYS` | Require every overlay file to exist. |
| `--prefix` | `ENVX_PREFIX` | Prefix prepended to every key. |
| `--suffix` | `ENVX_SUFFIX` | Suffix appended to every key. |
| `--delimiter` | `ENVX_DELIMITER` | String used to join list values. |
| `--namespace-prefix` | `ENVX_NAMESPACE_PREFIX` | Prefix each key with its namespace. |

Plus the [global flags](/commands/overview/#global-flags). The `--require-overlays`,
`--prefix`, `--suffix`, `--delimiter`, and `--namespace-prefix` flags map to
[settings](/configuration/schema/#settings-2) and override the values in `envx.yaml`.
