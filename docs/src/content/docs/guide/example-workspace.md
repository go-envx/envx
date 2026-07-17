---
title: Example Workspace
description: A complete, runnable envx workspace that shows every project, namespace, and setting.
sidebar:
  order: 3
---

This is a complete workspace that exercises every envx concept in one place: shared
and per-project namespaces, environment overlays, and a manifest that names every
setting. It is the companion to the [Configuration](/configuration/schema/) and
[Commands](/commands/overview/) references — skim those for detail, then scaffold
this to try things out:

```sh
envx create example-workspace
cd example-workspace
```

## The workspace

```
envx.yaml                          # the workspace manifest
env/                               # shared environment config used across projects
  database.yaml                    #   database namespace (nested -> DATABASE_*)
  database.development.yaml
  database.production.yaml
  gateway.yaml                     #   API gateway namespace (nested -> GATEWAY_*)
  gateway.development.yaml
  cache.yaml                       #   flat namespace (HOST/PORT) for the worker
  queue.yaml                       #   flat namespace (HOST/PORT) for the worker
api-service/                       # a backend API (FastAPI, Fastify, Express, Gin, ...)
  env/values.yaml                  #   the app's own config (flat -> NAME/PORT/LOG_LEVEL)
  env/values.development.yaml
web-server/                        # a web server (Nuxt, Next, SvelteKit, Caddy, ...)
  env/values.yaml
  env/values.development.yaml
```

## The manifest

The manifest names every setting in its global `settings` block at its default
value — a complete schema in one glance — then defines three projects:

```yaml
# envx.yaml
environments:
  - development
  - staging
  - production

# Every setting, shown at its default value.
settings:
  env:                     # defaults to the first environment (development)
  require_overlays: false
  prefix: ""
  suffix: ""
  namespace_prefix: false
  delimiter: ","
  overload: false

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
    settings:
      prefix: WEB          # a project-level override (see Settings Resolution)
  worker:
    includes:
      - env/cache
      - env/queue
```

- **`api-service`** composes the `database` and `gateway` namespaces with its own
  `values`.
- **`web-server`** shares `gateway` and adds a project-level `prefix` override — the
  seed for the [Settings Resolution](/configuration/settings-resolution/) examples.
- **`worker`** composes two flat namespaces that collide, to demonstrate
  [`namespace_prefix`](/configuration/schema/#namespace_prefix).

## The namespaces

### `database`

Shared by every project that talks to the database, `database` nests its keys
under the namespace name, so they resolve to `DATABASE_*` without needing
`namespace_prefix` — the recommended convention:

```yaml
# env/database.yaml
database:
  host: localhost
  port: 5432
  name: app
  user: app
  password: secret
  pool_hosts:
    - db-a.local
    - db-b.local
```

```yaml
# env/database.development.yaml
database:
  host: dev-db.local
  password: dev-secret
```

```yaml
# env/database.production.yaml
database:
  host: prod-db.internal
  user: prod_app
  password: prod-secret-rotated
```

There is deliberately **no `database.staging.yaml`** — that gap powers the
[`require_overlays`](/configuration/schema/#require_overlays) example.

### `gateway`

Shared by `api-service` and `web-server`, `gateway` likewise nests its keys, so
they resolve to `GATEWAY_*`:

```yaml
# env/gateway.yaml
gateway:
  url: http://gateway.local
  timeout: 30
```

```yaml
# env/gateway.development.yaml
gateway:
  url: http://localhost:8080
  timeout: 5
```

### `cache`

The `cache` and `queue` namespaces are deliberately **flat**, and both define
`host` and `port`. The `worker` project composes them, so their keys collide —
which is exactly where `namespace_prefix` earns its keep:

```yaml
# env/cache.yaml
host: cache.local
port: 6379
```

### `queue`

The other half of the `worker` collision — also flat, with the same `host` and
`port` keys that clash with `cache`:

```yaml
# env/queue.yaml
host: queue.local
port: 5672
```

### `api-service` values

Each app keeps its own config in a flat `values` namespace:

```yaml
# api-service/env/values.yaml
name: api-service
port: 3000
log_level: info
```

```yaml
# api-service/env/values.development.yaml
port: 3001
log_level: debug
```

### `web-server` values

```yaml
# web-server/env/values.yaml
name: web-server
port: 4000
```

```yaml
# web-server/env/values.development.yaml
port: 4001
```

## Resolve it

With the files in place, resolve values, inspect settings, or run a command:

```sh
# Print one resolved value (development is the default environment).
envx get api-service DATABASE_HOST
# dev-db.local

# Show where each resolved value came from.
envx explain api-service

# Run a command with the merged environment injected.
envx run api-service -- printenv
```

From here, the [Schema](/configuration/schema/#settings-2) page shows how each
setting changes resolution, and [Settings Resolution](/configuration/settings-resolution/)
shows how the same setting resolves across flags, environment variables, and
project and global config.
