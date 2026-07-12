---
title: Getting Started
description: Create a manifest, add environment files, and resolve them.
---

Create a manifest at your workspace root declaring your environments and what each
project loads:

```yaml
# envx.yaml
environments: [development, production]

projects:
  api:
    includes:
      - env/database
```

Each include is a **namespace**: a base file plus optional per-environment
overlays. Define a shared base and a production overlay:

```yaml
# env/database.yaml
host: localhost
credentials:
  password: dev-secret
```

```yaml
# env/database.production.yaml
host: db.internal
credentials:
  password: prod-secret
```

Then resolve or run against it:

```sh
envx get api HOST                  # localhost
envx get api HOST --env production # db.internal
envx explain api                   # show where each value came from
envx run api -- node server.js     # run with the merged environment
```

Nested keys flatten to `CREDENTIALS_PASSWORD`; lookups are case-insensitive.

:::tip
Read [Concepts](/guide/concepts/) to understand how namespaces, overlays, and
merging fit together, or jump to the [command reference](/reference/commands/).
:::
