<div align="center">

<a href="docs/src/content/docs/guide/getting-started.md">
  <img src="docs/src/assets/logo.png" alt="envx logo" width="200" height="200" />
</a>

# envx: Config/Secrets Manager

**Composable environment management for multi-project workspaces.**

[![Release](https://img.shields.io/github/v/release/go-envx/envx?sort=semver)](https://github.com/go-envx/envx/releases) [![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/) [![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

[Installation](docs/src/content/docs/guide/installation.mdx) &bull; [Getting Started](docs/src/content/docs/guide/getting-started.md) &bull; [Docs](docs/src/content/docs/configuration/overview.md) &bull; [Contributing](docs/src/content/docs/contributing.md)

</div>

---

Real projects rarely have one `.env` file. They have shared configs, per-app settings, and different values per environment. envx replaces that sprawl with a single declarative model: one manifest (`envx.yaml`) declares your environments and projects, and each project composes its environment from small, reusable files layered per environment, resolved with full visibility into where every value comes from.
