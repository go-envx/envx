---
title: Concepts
description: The building blocks of an envx workspace.
---

| Concept | Description |
| --- | --- |
| **Manifest** | `envx.yaml` at your workspace root. Declares environments, projects, and settings. Discovered by walking up from the working directory (stopping at the git root) unless `--config` / `ENVX_CONFIG` points at it. |
| **Environment** | A named variant such as `development` or `production`. The first one declared is the default. |
| **Project** | A named unit that composes an ordered list of namespaces into one resolved environment. |
| **Namespace** | A reusable slice of configuration: a base file `<name>.yaml` plus an optional per-environment overlay `<name>.<env>.yaml`. |
| **Merge** | Namespaces merge in declaration order (last wins). Within a namespace, the environment overlay is deep-merged over the base. Maps merge key-by-key; scalars and lists replace. |
| **Setting** | A knob controlling how files are loaded and keys are rendered (`env`, `strict`, `prefix`, `suffix`, `delimiter`, `namespace_prefix`, `overload`). |

## How values are resolved

For a given project and environment, envx:

1. Reads the project's `includes` in order.
2. For each namespace, loads the base `<name>.yaml` and deep-merges the
   environment overlay `<name>.<env>.yaml` on top of it.
3. Merges each namespace into the running result (later namespaces win).
4. Flattens nested keys into uppercase, underscore-joined env-var names
   (`credentials.password` → `CREDENTIALS_PASSWORD`) and joins list values into a
   single delimiter-separated string.

Settings that control this process are resolved separately. See
[Configuration](/reference/configuration/).
