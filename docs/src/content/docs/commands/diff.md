---
title: diff
description: Compare a project's resolved environment across two environments.
sidebar:
  order: 6
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

```sh
# Compare development against production.
envx diff api-core development production

# Reveal the differing values.
envx diff api-core development production --reveal

# Emit machine-readable output.
envx diff api-core development production --output json
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--reveal` | | Print values in plaintext instead of masking them. |
| `--output`, `-o` | | Output format: `table` (default) or `json`. |
| `--strict` | `ENVX_STRICT` | Require every overlay file to exist. |
| `--prefix` | `ENVX_PREFIX` | Prefix prepended to every key. |
| `--suffix` | `ENVX_SUFFIX` | Suffix appended to every key. |
| `--delimiter` | `ENVX_DELIMITER` | String used to join list values. |
| `--namespace-prefix` | `ENVX_NAMESPACE_PREFIX` | Prefix each key with its namespace. |

Plus the [global flags](/commands/overview/#global-flags). The `--strict`,
`--prefix`, `--suffix`, `--delimiter`, and `--namespace-prefix` flags map to
[settings](/configuration/settings-reference/) and override the values in `envx.yaml`.
