## Task
- [Task](https://taskfile.dev/) is the universal task runner for *all* apps and packages. Run `task --list` to see all available tasks.
- When creating new apps or packages, it is recommended that a `clean`, `check`, and `test` task be added in the projects `Taskfile.yaml`.
- *IMPORTANT*: Prior to running any command, you must refer to `task --list`. If it is possible to run the command via `task`, that is preferable.
- The root `Taskfile.yaml` imports all subdirectory taskfiles. Run imported tasks as `task <include>:<task-name>` (for example, `task envx:check`) instead of `task --dir app/ check`.
- Only run `docs:check` when files under `docs/` are updated; do not run them for changes outside that directory.

## Agent Workflow


### Code Quality
- **Always use the code-review agent after making changes** to review quality and correctness

### Skills Usage
Skills are defined in `./.agents/skills`. Unless specifically asked to ignore them, skills should be heavily utilized and selected based on relevancy to the current task. Below is a list of key skills and when they should be used. Other skills may exist and should also be used, as appropriate.
- **git-commit**: generates commit messages and opens pull request
- **code-review**: use this skill after making changes to review code quality
- **markdown-editor**: use this skill when writing, or editing Markdown files
