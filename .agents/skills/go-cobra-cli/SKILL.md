---
name: go-cobra-cli
description: "Best practices for developing Go Cobra-based CLI applications. Use when writing new commands, modifying existing CLI code, or reviewing Go CLI code for quality and correctness."
---

You are an expert Go developer specializing in CLI applications built with Cobra. Apply the following best practices when writing, modifying, or reviewing Go Cobra CLI code.

# Best Practices for Go Cobra CLI Development

## 1. Project Structure

```text
cmd/<binary>/main.go          # Minimal entrypoint — exit code handling only
internal/cmd/                 # Cobra command constructors
internal/app/                 # Application services and orchestration
internal/config/              # Configuration loading and validation
internal/runner/              # External process execution, if needed
```

- **Keep `main.go` minimal.** It should construct dependencies, execute the root command, and map errors to exit codes. Nothing else.
- **One file per command** under `internal/cmd/`. Each file defines a command constructor function.
- **Separate business logic from CLI wiring.** Cobra commands parse input and format output. Application logic lives in packages that have no knowledge of Cobra.

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    root := cmd.NewRootCommand(deps)
    root.SetContext(ctx)

    if err := root.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(exitCode(err))
    }
}
```

## 2. Command Design

- **Use command constructors**, not package-level variables. Return `*cobra.Command` from functions like `NewRunCommand(app *App)` so dependencies are injected and state is not shared.
- **Keep a consistent hierarchy.** Choose `[app] [noun] [verb]` or `[app] [verb] [noun]` and stick with it.
- **Prefer subcommands over overloaded flags.** `db migrate` is clearer than `db --action=migrate`.
- **Limit nesting to 2–3 levels.** Deeper hierarchies are hard to discover and use.
- **Treat command names as public API.** They become part of scripts and CI pipelines.

## 3. Always Use `RunE`

Use `RunE` instead of `Run` for every command that can fail. This enables:

- Centralized error handling at the process boundary.
- Clean testing without intercepting `os.Exit`.
- Consistent error formatting and exit-code mapping.

```go
RunE: func(cmd *cobra.Command, args []string) error {
    return app.Run(cmd.Context(), args[0], options)
}
```

**Never call `os.Exit()` from inside a command handler.**

## 4. Error Handling

- **Wrap errors with context:** `fmt.Errorf("load config %s: %w", path, err)`.
- **Make errors actionable.** Include the relevant input, path, or resource name.
- **Don't double-report.** Either print the error in the handler or return it — not both.
- **Follow Go conventions:** lowercase error strings, no trailing punctuation.
- **Silence usage on runtime errors.** By default, Cobra prints the full help on any error. Suppress this after parsing succeeds:

```go
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    cmd.SilenceUsage = true
    return nil
}
```

## 5. Exit Codes

- `0` — success.
- `1` — general runtime error.
- `2` — usage/validation error.
- Define additional codes for distinguishable failure modes (config error, not found, interrupted).
- Apply exit codes **only at the process boundary** (`main.go`), never deep inside command logic.
- Document exit codes for users who script the CLI.

## 6. Flags and Arguments

### Flags

- **Persistent flags** for options that apply globally or across subcommands (`--config`, `--verbose`, `--output`).
- **Local flags** for command-specific options (`--port` on `serve`).
- Mark required flags with `MarkFlagRequired`.
- Use `MarkFlagsRequiredTogether` and `MarkFlagsMutuallyExclusive` for flag relationships.
- Keep flag names long, explicit, and stable. Use shorthand only for very common options.
- Make defaults visible in help text.

### Arguments

- Use Cobra validators: `cobra.ExactArgs`, `cobra.MinimumNArgs`, or custom functions.
- Validate argument **meaning** separately from count (in `PreRunE` or early in `RunE`).
- Document argument names in `Use`: `deploy [environment] [service]`.
- Prefer subcommands over positional arguments with many modes.

## 7. Configuration Layering

Support a clear precedence chain: **Flags > Environment Variables > Config File > Defaults**.

- Bind flags to Viper with `viper.BindPFlag` in `PersistentPreRunE`.
- Keep Viper out of business logic — resolve config into typed structs before passing downstream.
- Use `$XDG_CONFIG_HOME` or `~/.config/<app>/` for config file location.
- Validate the **final resolved configuration**, not just individual sources.
- Make precedence behavior explicit in documentation.
- Don't let config loading fail for commands that don't need it (`help`, `version`, `completion`).

## 8. Context and Cancellation

- Use `cmd.Context()` inside `RunE` and pass it through to all application logic.
- Set up signal handling at the top level with `signal.NotifyContext`.
- Make long-running operations respect context cancellation.
- Use `exec.CommandContext` for subprocesses.
- Return a clear error when work is interrupted.

```go
ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
defer cancel()
```

## 9. Input and Output Design

- **stdout** — primary command output (data that users may pipe).
- **stderr** — diagnostics, progress, warnings, and human-facing status.
- Use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` so tests can capture output without touching real streams.
- Support `--output=json|yaml|table` for commands likely to be scripted.
- Respect `NO_COLOR` and suppress decorative output when stdout is not a terminal.
- Keep `--quiet`, `--verbose`, and `--debug` semantics distinct:
  - `--quiet`: suppress nonessential human output.
  - `--verbose`: additional operational detail.
  - `--debug`: diagnostic detail useful for bug reports.

## 10. Testing

- **Test commands through constructed instances**, not by executing the compiled binary:

```go
func executeCommand(root *cobra.Command, args ...string) (string, string, error) {
    stdout := new(bytes.Buffer)
    stderr := new(bytes.Buffer)
    root.SetOut(stdout)
    root.SetErr(stderr)
    root.SetArgs(args)
    err := root.Execute()
    return stdout.String(), stderr.String(), err
}
```

- **Test business logic separately from Cobra.** Core logic should be testable without constructing commands.
- Use **table-driven tests** for flag parsing, argument validation, and output formatting.
- Test invalid flag combinations, missing required flags, and argument count violations.
- Use temporary directories for filesystem operations.
- Avoid tests that depend on global process state (real env vars, cwd, real stdout).
- Mock external dependencies via small interfaces injected through command constructors.

## 11. Dependency Injection

- Inject filesystem access, environment readers, clocks, HTTP clients, and process runners.
- Avoid direct calls to `os.Getenv`, `os.Getwd`, `time.Now`, and `exec.Command` scattered throughout command logic.
- Keep interfaces small and owned by the consuming package.
- Use concrete types unless an interface provides clear testability value.
- Construct all dependencies in `main.go` or `PersistentPreRunE` and pass them into command constructors.

## 12. Help and Documentation

- **`Short`** — concise phrase (< 50 chars) shown in parent command listings.
- **`Long`** — fuller description shown on `--help` for the specific command.
- **`Example`** — realistic, copy-pasteable usage examples on every user-facing command.

```go
Example: `  myapp deploy staging
  myapp deploy production --dry-run --force`
```

- Use Cobra's `doc.GenMarkdownTree` or `doc.GenManTree` to auto-generate reference documentation.
- Review generated help output as part of development. Help text is UX.

## 13. Shell Completions

- Ship a `completion` subcommand — Cobra provides built-in support for bash, zsh, fish, and PowerShell.
- Add `ValidArgsFunction` for dynamic completions (e.g., completing resource names).
- Avoid slow network calls in completion functions; respect context cancellation.
- Test completion behavior for commands with complex arguments.

## 14. Version and Build Metadata

- Provide a `version` subcommand or `--version` flag.
- Embed version, commit, and build date via `-ldflags`:

```go
var (
    version = "dev"
    commit  = "unknown"
    date    = "unknown"
)
```

- Include Go version and OS/arch in verbose version output.
- Use `debug.ReadBuildInfo()` as a fallback for module version in development builds.

## 15. Automation and Interactive UX

- **Assume the CLI will be scripted.** Keep machine-readable output formats stable.
- Provide `--yes` or `--force` for commands that would otherwise prompt interactively.
- Detect non-interactive stdin (`!term.IsTerminal(os.Stdin.Fd())`) and fail clearly rather than blocking on input.
- Confirm destructive actions unless force/yes is provided.
- Provide `--dry-run` for risky or destructive operations.
- Make failures deterministic — same input should produce same error behavior.

## 16. Security

- Treat all CLI input as untrusted.
- Pass subprocess arguments as separate strings — never through shell interpolation.
- Validate paths before destructive operations; be cautious with symlinks.
- Redact secrets in logs, errors, and debug output.
- Use least-privilege file permissions for sensitive data (e.g., `0600`).
- Never write credentials to config files unless explicitly requested.

## 17. Backward Compatibility

- Treat command names, flags, config keys, output formats, and exit codes as compatibility surfaces.
- Use Cobra's deprecation support when removing or renaming flags:

```go
cmd.Flags().Bool("old-flag", false, "")
_ = cmd.Flags().MarkDeprecated("old-flag", "use --new-flag instead")
```

- Keep deprecated behavior working long enough for users to migrate.
- Mention replacements clearly in deprecation messages.

## 18. Performance

- Keep startup fast. Avoid expensive work during package initialization or `init()` functions.
- Use `cobra.OnInitialize()` for deferred setup.
- Don't perform network calls before command validation succeeds.
- Lazy-load heavy configuration only for commands that need it.
- Stream large data instead of buffering everything in memory.

## 19. Distribution

- Build static binaries with `CGO_ENABLED=0` for portability.
- Cross-compile with `GOOS`/`GOARCH` in CI.
- Ship checksums for release artifacts.
- Sign releases when supply-chain assurance matters.
- Verify that packaged binaries contain correct version metadata.

---

## Quick Reference: Common Pitfalls

| Pitfall | Fix |
|---------|-----|
| `os.Exit()` inside handlers | Return errors; exit only in `main.go` |
| Global mutable command variables | Use constructor functions with DI |
| Printing error AND returning it | Do one or the other |
| Business logic inside `RunE` | Delegate to application packages |
| Usage printed on runtime errors | `SilenceUsage = true` after parsing |
| Persistent flags where local flags suffice | Scope flags to the command that needs them |
| Writing to `fmt.Println` directly | Use `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` |
| Expensive init before knowing which command runs | Lazy-load in `PersistentPreRunE` |
| Config loading fails `help`/`version` commands | Only load config for commands that need it |
| Ignoring context cancellation | Thread `cmd.Context()` through all operations |

---

## Development Checklist

Before shipping a command, verify:

- [ ] Command name, aliases, and hierarchy are intentional
- [ ] `Use`, `Short`, `Long`, and `Example` are clear and accurate
- [ ] Argument count and meaning are validated
- [ ] Flags have clear names, defaults, and validation
- [ ] Incompatible flag combinations are rejected
- [ ] Business logic lives outside Cobra wiring
- [ ] Output goes through command-provided writers
- [ ] Errors are actionable and not duplicated
- [ ] Context cancellation is respected
- [ ] Tests cover success, validation failure, runtime failure, and output
- [ ] Help output has been reviewed
- [ ] Automation/scripting behavior is predictable
- [ ] Secrets are never logged or printed
- [ ] Exit codes are intentional and documented

---

## Appendix: General Go Practices (Applicable to CLI Development)

These are not Cobra-specific but are consistently important when building CLI tools in Go:

- **Error strings are lowercase** and don't end with punctuation (`"open file: permission denied"`, not `"Failed to open file."`).
- **Wrap errors with `%w`** to preserve the chain for callers that need `errors.Is` / `errors.As`.
- **Use sentinel or typed errors sparingly** — only when callers must branch on them.
- **Keep interfaces small** (1–3 methods) and define them in the consuming package, not the provider.
- **Prefer concrete types** unless an interface provides clear testability or decoupling value.
- **Avoid `init()` functions.** Explicit initialization in `main` or constructors is easier to reason about and test.
- **Use `context.Context` as the first parameter** for any function that does I/O, calls external services, or may need cancellation.
- **Table-driven tests** reduce duplication for parsing, validation, and formatting logic.
- **Use `t.TempDir()`** for filesystem tests — it handles cleanup automatically.
- **Stream data** rather than buffering entire payloads in memory when the size is unbounded.
- **Atomic file writes** (write to temp, then rename) prevent corruption on crash for important files.
- **Prefer `exec.CommandContext`** over `exec.Command` so subprocesses are killed on cancellation.
- **Never interpolate user input into shell strings.** Pass arguments as discrete elements to `exec.Command`.
- **Validate at boundaries** (CLI input, config files, network responses) — internal code can then trust its inputs.
