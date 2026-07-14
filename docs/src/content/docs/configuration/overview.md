---
title: Overview
description: How the envx.yaml manifest is structured and discovered.
sidebar:
  order: 1
---

Configuration lives in a single file, `envx.yaml`, at your workspace root. It declares your environments, your projects, and any global or per-project settings. This page covers how that file is shaped and how envx finds it. For a reference of every individual setting, see [Settings Reference](/configuration/settings-reference/); for how settings resolve when the same one is set in more than one place, see [Settings Resolution](/configuration/settings-resolution/).

## The envx.yaml manifest

A manifest has three top-level sections:

```yaml
# envx.yaml
environments:
  - development
  - staging
  - production

settings:
  strict: true          # global settings apply to every project

projects:
  api:
    includes:
      - env/database
      - env/gateway
      - api/env/values
    settings:
      strict: false     # project settings override the global ones
  web:
    includes:
      - env/gateway
      - web/env/values
    settings:
      prefix: WEB
```

Each top-level section plays a distinct role:

| Section | Purpose |
| --- | --- |
| `environments` | The list of environments you can resolve against. The first entry is the default when none is selected. |
| `settings` | Global settings that apply to every project. |
| `projects` | A map of project name to definition. Each project has an ordered `includes` list of namespaces plus optional project-level `settings`. Namespaces merge in declaration order, so when two define the same key, the one listed last wins. |

## How envx.yaml is discovered

envx finds its manifest in one of two ways:

1. **Automatic Discovery** — By default, envx walks up from the current working directory, stopping at the git repository root, and uses the first file named `envx.yaml` it finds.

2. **Explicit Path** — To use a file in another location, or one that is not named `envx.yaml`, point at it directly with the `--config` flag or the `ENVX_CONFIG` environment variable:

   ```sh
   envx get api HOST --config ./path/to/my-config.yaml
   ENVX_CONFIG=./path/to/my-config.yaml envx get api HOST
   ```

Continue to [Settings Reference](/configuration/settings-reference/) for the full list of settings you can place in `envx.yaml`, then [Settings Resolution](/configuration/settings-resolution/) for how those settings are resolved when the same one is set in more than one place.
