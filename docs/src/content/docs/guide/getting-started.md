---
title: Getting Started
description: Learn the key concepts, then build your first envx workspace.
---

This guide introduces the core ideas envx is built on and then walks through setting up a project from scratch.

## Key concepts

envx works with three kinds of files:

| File | Role |
| --- | --- |
| `envx.yaml` | The workspace configuration file: defines the environments, projects, and any global settings for envx to use. By default, this file lives at your workspace root. |
| `<namespace>.yaml` | The base layer for a namespace: a reusable slice of environment values shared by every environment. |
| `<namespace>.<environment>.yaml` | The environment overlay for a namespace: values that override or extend the base for one specific environment. |

A few terms are worth naming up front:

- An **environment** is a named variant such as `development` or `production`.
- A **project** composes an ordered list of **namespaces** into one resolved
  environment.
- A **namespace** is a base file plus its optional per-environment overlays.

That is the whole model. Everything else is detail you can pick up as you go.

## Set up a project

### 1. Workspace configuration file

Create an `envx.yaml` at your workspace root. It lists your environments and the
namespaces each project loads:

```yaml
# envx.yaml
environments:
  - development
  - production

projects:
  api-service:
    includes:
      - env/database
      - env/gateway
      - api-service/env/values
  web-server:
    includes:
      - env/gateway
      - web-server/env/values
```

Each entry under `includes` is a namespace. They merge in declaration order,
so if two namespaces define the same key, the one listed last wins. Here
`api-service/env/values` would override any key it shares with `env/database`.

### 2. Base namespace files

For each namespace, create a base `<namespace>.yaml`. It declares every key the
namespace provides, with defaults for shared values and blanks for anything an
environment must supply:

```yaml
# env/database.yaml
database:
  host: localhost
  port: 5432
```

```yaml
# env/gateway.yaml
gateway:
  url:
  timeout: 30
```

```yaml
# api-service/env/values.yaml
name: api-service
port: 4000
log:
  level: info
  format: json
```

```yaml
# web-server/env/values.yaml
name: web-server
port: 3000
```

**Things to note:**

- Nested keys flatten into `SCREAMING_SNAKE_CASE` env vars, so `log.level`
  becomes `LOG_LEVEL`.
- Lookups are case-insensitive.

### 3. Environment overlay files

Add a `<namespace>.<environment>.yaml` overlay for any values that differ per
environment. Each overlay is merged over its base, so you only write what changes:

```yaml
# env/database.production.yaml
database:
  host: db.production.host
```

```yaml
# env/gateway.development.yaml
gateway:
  url: http://localhost
```

```yaml
# env/gateway.production.yaml
gateway:
  url: https://gateway.example.com
```

```yaml
# api-service/env/values.production.yaml
log_level: warn
```

**Things to note:**

- Overlays supply only the keys that change, so in `production` just
  `DATABASE_HOST`, `GATEWAY_URL`, and `LOG_LEVEL` come from these files. Every
  other key still resolves from its base.
- Flat and nested spellings resolve to the same variable, so the api-service
  overlay's flat `log_level` overrides its nested `log.level` base: both become
  `LOG_LEVEL`.
- A namespace needs an overlay only when its values change, so the web server has
  no `web-server/env/values.production.yaml`.

### 4. Resolve and run

With the files in place, resolve values, inspect them, or run a command with the
merged environment injected:

```sh
envx get api-service DATABASE_HOST
# localhost

envx get api-service DATABASE_HOST --env production
# db.production.host

envx get web-server GATEWAY_URL
# http://localhost

envx get web-server DATABASE_HOST
# error: key "DATABASE_HOST" not found

envx explain web-server
# show where each value came from

envx run api-service -- node server.js
# run with the merged environment

envx run web-server -- node server.js
# gateway + web-server values only
```

Each project resolves only the namespaces in its `includes` block, so values stay
scoped to the project that asks for them.

---

:::tip[Next steps]
- Learn how to shape `envx.yaml` and every available setting in
  [Configuration](/configuration/overview/).
- See the full command surface, with examples and flags, in
  [Commands](/commands/overview/).
:::
