---
title: run
description: Run a command with a project's merged environment injected.
sidebar:
  order: 4
---

```sh
envx run <project> -- <command> [args...]
```

`run` executes a command with the environment variables resolved from a project's
namespace chain. Everything after the `--` separator is the command to run and
its arguments.

By default, existing OS environment variables take precedence over values from
your files. Pass `--overload` to let file values win instead. Signals are
forwarded transparently to the child process, and its exit code is propagated
verbatim.

## Arguments

| Argument | Description |
| --- | --- |
| `<project>` | The project whose environment is injected. |
| `<command> [args...]` | The command to run, after a `--` separator. |

## Examples

These examples use the [Example Workspace](/guide/example-workspace/) (`envx create example-workspace`).

```sh
# Run a command with the default environment.
envx run api-service -- npm start

# Resolve against a specific environment.
envx run api-service --env production -- node server.js

# Let file values override existing OS env vars.
envx run api-service --overload -- npm start
```

## Flags

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--env`, `-E` | `ENVX_ENV` | Target environment to resolve. |
| `--overload` | `ENVX_OVERLOAD` | File values override existing OS env vars. |
| `--require-overlays` | `ENVX_REQUIRE_OVERLAYS` | Require every overlay file to exist. |
| `--prefix` | `ENVX_PREFIX` | Prefix prepended to every key. |
| `--suffix` | `ENVX_SUFFIX` | Suffix appended to every key. |
| `--delimiter` | `ENVX_DELIMITER` | String used to join list values. |
| `--namespace-prefix` | `ENVX_NAMESPACE_PREFIX` | Prefix each key with its namespace. |

Plus the [global flags](/commands/overview/#global-flags). These flags map to
[settings](/configuration/schema/#settings-2) and override the values in `envx.yaml`.

## Exit codes

envx maps outcomes to conventional exit codes so `run` composes cleanly in
scripts:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Runtime error |
| `2` | Usage or validation error (invalid flags or arguments) |
| `126` | Command found but not executable |
| `127` | Command not found |
| `128+N` | Child terminated by signal `N` (e.g. `130` for SIGINT) |

The child process's own exit code is propagated verbatim, so a command that exits
`7` makes envx exit `7`.
