---
title: Settings Resolution
description: How envx resolves each setting when the same one is supplied in more than one place.
sidebar:
  order: 3
---

The same setting can be declared in several places — a global default in
`envx.yaml`, a per-project override, a CLI flag, or an environment variable. This
page explains how envx decides which one wins. For what each setting does, see
[Schema](/configuration/schema/#settings-2).

## Per-project overrides

Settings can be declared globally and overridden for a single project. A project
setting always wins over the matching global setting. The
[Example Workspace](/guide/example-workspace/) does exactly this: its `web-server` project
sets `prefix: WEB`, overriding the global default of `""`.

## Precedence

The same settings can also be supplied at the command line or through the
environment. envx resolves each setting using this precedence, highest to lowest:

1. **CLI flag** (e.g. `--prefix`)
2. **`ENVX_*` environment variable** (e.g. `ENVX_PREFIX`)
3. **Project settings** (`projects.<name>.settings`)
4. **Global settings** (top-level `settings`)
5. **Built-in default**

## Examples

Scaffold the [Example Workspace](/guide/example-workspace/) (`envx create example-workspace`),
where the `web-server` project sets `prefix: WEB` (a project override of the global
default `""`). Watch that project setting get overridden in turn by an
`ENVX_PREFIX` variable and then a `--prefix` flag — each layer changes the prefix
applied to every resolved key:

```sh
# Project setting: keys are WEB_-prefixed.
envx run web-server -- printenv WEB_GATEWAY_URL
# http://localhost:8080

# ENVX_PREFIX overrides the project setting: keys become API_-prefixed.
ENVX_PREFIX=API envx run web-server -- printenv API_GATEWAY_URL
# http://localhost:8080

# A --prefix flag overrides the variable: keys become CLI_-prefixed.
ENVX_PREFIX=API envx run web-server --prefix cli -- printenv CLI_GATEWAY_URL
# http://localhost:8080
```
