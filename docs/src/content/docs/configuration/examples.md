---
title: Examples
description: One sample workspace, then a section per setting showing how it changes command output.
sidebar:
  order: 4
---

Every setting below runs against the same small workspace, so you can see exactly how each one changes what envx resolves. Recreate these files to follow along, then jump to any setting.

## The sample workspace

```
envx.yaml
env/
  postgres.yaml
  postgres.development.yaml
  postgres.production.yaml
  gateway.yaml
  gateway.development.yaml
apps/
  api-core/env/api-core.yaml
  api-core/env/api-core.development.yaml
```

The manifest declares three environments and an `api-core` project that composes three namespaces:

```yaml
# envx.yaml
environments: [development, staging, production]

projects:
  api-core:
    includes:
      - env/postgres
      - env/gateway
      - apps/api-core/env/api-core
```

Each namespace has a base file plus per-environment overlays:

```yaml
# env/postgres.yaml
host: localhost
port: 5432
credentials:
  username: postgres
  password: secret
```

```yaml
# env/postgres.development.yaml
host: dev-db.local
credentials:
  password: dev-secret
```

```yaml
# env/postgres.production.yaml
host: prod-db.internal
credentials:
  username: prod_user
  password: prod-secret-rotated
```

```yaml
# env/gateway.yaml
url: http://gateway.local
timeout: 30
```

```yaml
# env/gateway.development.yaml
url: http://localhost:8080
timeout: 5
```

```yaml
# apps/api-core/env/api-core.yaml
app_name: api-core
port: 3000
log_level: info
```

```yaml
# apps/api-core/env/api-core.development.yaml
log_level: debug
port: 3001
```

A few things to keep in mind while reading the examples:

- `development` is declared first, so it's the **default environment** when `--env` is omitted.
- There is deliberately **no `postgres.staging.yaml`** — that gap drives the `strict` example.
- Both `postgres` and `api-core` define `port`. Because `api-core` is included last, its value wins — until `namespace_prefix` keeps them apart.

## env

`env` selects which `<namespace>.<environment>.yaml` overlays are layered on top of each base file, so the same lookup resolves a different value per environment:

```sh
envx get api-core HOST
dev-db.local

envx get api-core HOST --env production
prod-db.internal
```

## strict

By default a missing overlay is skipped and the base value is used. `strict` turns that gap into an error instead. Since there is no `postgres.staging.yaml`:

```sh
envx get api-core HOST --env staging
localhost

envx get api-core HOST --env staging --strict
Error: loading environment file env/postgres.staging.yaml: open env/postgres.staging.yaml: no such file or directory
```

## prefix

`prefix` prepends a string — uppercased and joined with an underscore — to every resolved key, so `HOST` is exposed as `APP_HOST`:

```sh
envx get api-core HOST
dev-db.local

envx get api-core APP_HOST --prefix app
dev-db.local
```

## suffix

`suffix` appends a string the same way, so `--suffix v2` turns `HOST` into `HOST_V2`:

```sh
envx get api-core HOST_V2 --suffix v2
dev-db.local
```

## namespace_prefix

`namespace_prefix` prepends each key with the name of the namespace it came from. It's the cleanest way to resolve collisions: `postgres` and `api-core` both define `port`, so normally the last include wins and the other is lost:

```sh
envx get api-core PORT
3001
```

Turn it on and the two keys stay distinct, as `POSTGRES_PORT` and `API_CORE_PORT`:

```sh
envx get api-core POSTGRES_PORT --namespace-prefix
5432

envx get api-core API_CORE_PORT --namespace-prefix
3001
```

## delimiter

A YAML list is joined into a single value. Suppose `postgres.yaml` also declared one:

```yaml
# env/postgres.yaml
read_replicas:
  - db1.local
  - db2.local
  - db3.local
```

`delimiter` chooses the separator, a comma by default:

```sh
envx get api-core READ_REPLICAS
db1.local,db2.local,db3.local

envx get api-core READ_REPLICAS --delimiter ';'
db1.local;db2.local;db3.local
```

## overload

`run` injects the resolved environment into a child process. By default an existing OS variable wins; `overload` lets the file value take over instead. With `HOST` already set in the shell:

```sh
HOST=os-host envx run api-core -- printenv HOST
os-host

HOST=os-host envx run api-core --overload -- printenv HOST
dev-db.local
```
