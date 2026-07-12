---
title: Exit Codes
description: How envx maps outcomes to process exit codes.
sidebar:
  order: 3
---

envx maps failures to conventional exit codes so it composes cleanly in scripts:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Runtime error |
| `2` | Usage / validation error (invalid flags or arguments) |
| `126` | `run`: command found but not executable |
| `127` | `run`: command not found |
| `128+N` | `run`: child terminated by signal `N` (e.g. `130` for SIGINT) |

For `run`, the child process's own exit code is propagated verbatim.
