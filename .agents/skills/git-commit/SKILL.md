---
name: git-commit
description: Generates well-formatted git commit messages following conventional commit standards. Auto-detects branch context to produce informal WIP commits for feature branches or polished conventional commits for main branch merges. Use when committing code changes or preparing commits for merge.
---

You are an expert at crafting clear, meaningful git commit messages.

## Commit Modes

The skill auto-detects the appropriate mode based on the current branch:

| Branch Pattern | Mode | Style |
|----------------|------|-------|
| `main`, `master`, `develop` | **Formal** | Polished conventional commit |
| `feature/*`, `fix/*`, `chore/*`, etc. | **Quick** | Concise work-in-progress |

Override with explicit flags: `--formal` or `--quick`

## Workflow

1. Run `git branch --show-current` to detect current branch
2. Run `git diff --staged` to analyze staged changes (if empty, check `git diff`)
3. Determine mode based on branch or user override
4. Generate appropriate commit message
5. Present the commit command for user confirmation

## Conventional Commit Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Type Selection

| Type | When to Use | SemVer |
|------|-------------|--------|
| `feat` | New feature or capability | MINOR |
| `fix` | Bug fix | PATCH |
| `docs` | Documentation only | - |
| `style` | Formatting, no logic change | - |
| `refactor` | Code restructuring, same behavior | - |
| `perf` | Performance improvement | - |
| `test` | Adding or updating tests | - |
| `build` | Build system, dependencies | - |
| `chore` | Maintenance, tooling | - |

### Decision Flowchart

1. Does it fix a bug? → `fix`
2. Does it add/change a feature? → `feat`
3. Does it improve performance? → `perf`
4. Does it restructure code without behavior change? → `refactor`
5. Is it formatting only? → `style`
6. Is it test-related? → `test`
7. Is it documentation only? → `docs`
8. Is it build/dependency related? → `build`
9. Otherwise → `chore`

## Writing Rules

- Use **imperative present tense**: "add" not "added" or "adds"
- **No capitalization** of first letter in description
- **No period** at end of description
- Keep description under 72 characters
- **Breaking changes**: Add `!` after type/scope (e.g., `feat!:`)

## Quick Mode (Feature Branch)

For work-in-progress commits, generate concise messages:

```
<type>: <brief description>
```

Examples:
- `feat: add password reset form`
- `fix: correct validation logic`
- `wip: auth flow in progress`

## Formal Mode (Main Branch)

For polished commits, include body when helpful:

```
feat(auth): add password reset functionality

- Add forgot password form with email validation
- Implement verification token generation
- Add password reset API endpoint
- Include rate limiting for security

Closes #123
```

## Output Format

Present the generated message with a ready-to-run command:

```
**Generated Commit Message:**

feat(auth): add password reset functionality

- Add forgot password form with email validation
- Implement verification token generation

**Run this command to commit:**
git commit -m "feat(auth): add password reset functionality" -m "- Add forgot password form with email validation" -m "- Implement verification token generation"
```

For multi-line messages, use multiple `-m` flags or suggest the user run `git commit` to open their editor with the message.
