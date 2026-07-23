# envx docs

The documentation site for envx, built with [Astro](https://astro.build/) and [Starlight](https://starlight.astro.build/).

Content lives in [src/content/docs/](src/content/docs/) as Markdown (`.md`) and MDX (`.mdx`) files.

## Tasks

All tasks run through [Task](https://taskfile.dev/) and are namespaced under `docs:` from the repository root.

| Task | Purpose |
| --- | --- |
| `task docs:dev` | Start the local dev server |
| `task docs:build` | Build the static site to `dist/` |
| `task docs:preview` | Build, then preview the built site |
| `task docs:check` | Format, lint, and type-check |
| `task docs:clean` | Remove build artifacts and dependencies |
