---
title: Introduction
description: What envx is and the problem it solves.
---

Real projects rarely have one `.env` file. They have a database config shared by
several services, a gateway config, per-app settings, and different values for
development, staging, and production. Managing that with copy-pasted `.env` files
quickly becomes error-prone: values drift, overrides are invisible, and nobody
knows which file actually won.

envx replaces that sprawl with a single, declarative model: one manifest declares
your environments and projects, and each project composes its environment from
small, reusable files layered per environment.

## Features

- **Workspace manifest**: declare environments, projects, and settings in one
  place.
- **Composable namespaces**: build a project's environment from an ordered list
  of reusable files, merged with last-one-wins semantics.
- **Base + overlay files**: a `<name>.yaml` base plus optional
  `<name>.<env>.yaml` overlays keep environment differences small and explicit.
- **Explain & diff**: see where every value resolves from, and compare a project
  across two environments.
- **`run`**: launch any command with the fully merged environment injected, with
  transparent signal forwarding and exit-code propagation.

Ready to try it? Head to [Installation](/guide/installation/) and then
[Getting Started](/guide/getting-started/).
