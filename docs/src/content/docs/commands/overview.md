---
title: Overview
description: The envx command surface and the flags shared across commands.
sidebar:
  order: 1
---

The envx CLI is a small set of commands for resolving, inspecting, and running
your projects' environments. Each command has its own page with examples and its
full flag list.

| Command | Purpose |
| --- | --- |
| [`get`](/commands/get/) | Resolve a project's environment and print one value. |
| [`run`](/commands/run/) | Run a command with the merged environment injected. |
| [`set`](/commands/set/) | Write a key/value into a namespace's overlay file. |
| [`explain`](/commands/explain/) | Show each resolved value and the file it came from. |
| [`diff`](/commands/diff/) | Compare a project's environment across two environments. |

## Global flags

These flags are available on every command:

| Flag | Environment variable | Description |
| --- | --- | --- |
| `--config` | `ENVX_CONFIG` | Path to `envx.yaml`. Overrides auto-discovery. |
| `--help`, `-h` | | Show help for the command. |
| `--version` | | Print the envx version (root command). |

## Settings flags

Most commands also accept flags that map to [settings](/configuration/settings-reference/),
such as `--env`, `--strict`, `--prefix`, `--suffix`, `--delimiter`, and
`--namespace-prefix`. These override the matching value in `envx.yaml`. Each
command page lists exactly which ones it accepts.
