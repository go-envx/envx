---
title: Settings Resolution
description: How envx resolves each setting when the same one is supplied in more than one place.
sidebar:
  order: 3
---

The same setting can be declared in several places — a global default in `envx.yaml`, a per-project override, a CLI flag, or an environment variable. This page explains how envx decides which one wins. For what each setting does, see [Settings Reference](/configuration/settings-reference/).

## Per-project overrides

Settings can be declared globally and overridden for a single project. A project setting always wins over the matching global setting:

```yaml
environments:
  - development
  - staging
  - production

settings:
  strict: true        # global settings apply to every project

projects:
  api:
    includes:
      - env/database
      - env/gateway
      - api/env/values
    settings:
      strict: false   # this project only; strict=false
  web:
    includes:
      - env/gateway
      - web/env/values
    settings:         # inherits strict=true
      prefix: NUXT

```

## Override precedence

The same settings can also be supplied at the command line or through the environment. envx resolves each setting using this precedence, highest to lowest:

1. **CLI flag** (e.g. `--strict`)
2. **`ENVX_*` environment variable** (e.g. `ENVX_STRICT=true`)
3. **Project settings** (`projects.<name>.settings`)
4. **Global settings** (top-level `settings`)
5. **Built-in default**
