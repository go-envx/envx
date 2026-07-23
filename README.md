<div align="center">

<a href="docs/src/content/docs/guide/getting-started.mdx">
  <img src="docs/src/assets/logo.png" alt="envx logo" width="200" height="200" />
</a>

# envx: Config Manager

**Composable environment management for multi-project workspaces.**

[![Release](https://img.shields.io/github/v/release/go-envx/envx?sort=semver)](https://github.com/go-envx/envx/releases) [![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/) [![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

[Installation](docs/src/content/docs/guide/installation.mdx) &bull; [Getting Started](docs/src/content/docs/guide/getting-started.mdx) &bull; [Docs](docs/src/content/docs/configuration/schema.mdx) &bull; [Contributing](docs/src/content/docs/contribute.mdx)

</div>

---

Real projects rarely have one `.env` file. They have shared configs, per-app settings, and different values per environment. envx replaces that sprawl with a single, declarative model: one manifest (`envx.yaml`) declares your environments and projects, and each project composes its environment by splitting config into natural namespaces and layering per-environment overrides on top of base defaults, resolved with full visibility into where every value comes from.
