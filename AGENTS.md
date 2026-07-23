## Task
- [Task](https://taskfile.dev/) is the universal task runner for *all* apps and packages. Run `task --list` to see all available tasks.
- When creating new apps or packages, it is recommended that a `clean`, `check`, and `test` task be added in the projects `Taskfile.yaml`.
- *IMPORTANT*: Prior to running any command, you must refer to `task --list`. If it is possible to run the command via `task`, that is preferable.

## Go Cobra CLI Development
- When writing, modifying, or reviewing code in the Go Cobra CLI app (`app/`), **always** read and apply the `go-cobra-cli` skill (`.agents/skills/go-cobra-cli/SKILL.md`).
- When performing code reviews on Go CLI code, use the `go-cobra-cli` skill as the quality standard in addition to the `code-review` skill.

## Markdown Formatting
- When writing or modifying Markdown files (`.md` or `.mdx`), do not add hard line breaks to manually wrap text. Each paragraph and list item must remain a single continuous line of text, relying on the IDE's auto text-wrapping feature instead.
