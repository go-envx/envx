---
name: git-commit
description: Commits changes to a new branch and opens a pull request in one step, for a squash-merge workflow where the PR title and description become the commit on main. Writes the PR title and body as a conventional-commit message in plain text (not a markdown document), and moves review notes, test plans, and checklists into a separate PR comment. Use when preparing changes for review, opening a PR, or shipping a branch toward a protected main branch.
---

You commit changes to a new branch and open a pull request as one motion, for a repository where `main` is protected and pull requests are always squash-merged.

## The one thing to understand

Because this repo squash-merges, the **PR title and description become the commit on `main`**. The individual commits on the feature branch are squashed away and discarded. So the PR title and body are not documentation *about* a commit — they *are* the commit message.

This has one consequence that governs everything below:

**Write the PR body as a git commit message, not as a markdown page.** Plain text, wrapped, terse, explaining *why*. Anything that reads like documentation — a test plan, a reviewer walkthrough, screenshots, a checklist — does not belong in the body, because it would land verbatim in `main`'s history. That material goes in a **separate PR comment** instead.

If you find yourself reaching for a `## Summary` heading, stop: that content is either a plain commit body or a PR comment.

## Assumptions

- `main` is protected; nothing is committed directly to it.
- Every merge is a squash merge.
- The repo's squash default is set to **"Pull request title and description"** (Settings → General → Pull Requests). If it isn't, recommend the user set it — it's what makes the PR title/body flow into the commit automatically.

## Workflow

1. Run `git branch --show-current` and `git status` to see where things stand.
2. **Never commit on `main`/`master`.** If the current branch is protected, create a feature branch first with `git switch -c <branch>` (uncommitted changes carry over). Choose the name from the change itself: `<type>/<short-kebab-description>`, e.g. `feat/user-auth`, `fix/login-validation`.
3. Inspect the change: `git diff --staged` (or `git diff` if nothing is staged), plus `git diff main...HEAD --stat` if commits already exist on the branch.
4. Compose **one** conventional-commit message — a title and, if warranted, a short body. This single message is the source of truth for both the branch commit and the PR. Do not author two different versions.
5. Stage and commit on the branch with that message.
6. Push: `git push -u origin <branch>`.
7. Open the PR with **title = the commit subject** and **body = the commit body** (plain text).
8. If there is review scaffolding (test plan, QA steps, screenshots, reviewer guidance, open questions), post it as a **separate PR comment** — never in the body.
9. Present the commands for confirmation before running anything that pushes or creates a PR. Do not push or open the PR unprompted.

## The message is the commit

**Title** — conventional commit format:

```
<type>(<scope>): <description>
```

- imperative mood: "add", not "added" or "adds"
- lowercase description, no trailing period
- aim for ≤ 50 characters, 72 hard limit
- `<scope>` is optional; include it when it sharpens the subject

**Body** (optional, only when the *why* isn't obvious from the title):

- plain text, wrapped at ~72 characters
- explain the motivation and the effect, not the mechanics — the diff already shows *how*
- plain hyphen bullets are fine for enumerating a few distinct logical changes; keep them terse
- **no** markdown headings, bold, tables, or `## Summary / ## Changes / ## Testing` sections
- if the body runs past ~5–8 lines, the overflow is review scaffolding — move it to a comment

**Footer** (optional): `Closes #NN`, `BREAKING CHANGE: <description>`, `Co-authored-by: Name <email>`.

## Type selection

| Type | When to use | SemVer |
|------|-------------|--------|
| `feat` | New feature or capability | MINOR |
| `fix` | Bug fix | PATCH |
| `perf` | Performance improvement | PATCH |
| `refactor` | Restructuring, same behavior | – |
| `docs` | Documentation only | – |
| `test` | Adding or updating tests | – |
| `build` | Build system or dependencies | – |
| `style` | Formatting only, no logic change | – |
| `chore` | Maintenance, tooling | – |

Decision order: fixes a bug → `fix`; adds/changes a feature → `feat`; improves performance → `perf`; restructures without behavior change → `refactor`; formatting only → `style`; tests → `test`; docs → `docs`; build/deps → `build`; otherwise → `chore`.

**Breaking changes:** add `!` after the type/scope (`feat(api)!:`) and add a `BREAKING CHANGE:` footer describing the migration.

## Review scaffolding goes in a comment

Anything a reviewer needs but `main`'s history does not: test plans, manual QA steps, screenshots or GIFs, "look at X first" notes, open questions, rollout or revert notes, checklists.

After the PR exists, post it as a comment:

```bash
gh pr comment --body-file - <<'EOF'
Testing
- unit tests for token generation and expiry
- manual QA: requested reset, followed the email link, set a new
  password on Chrome and Firefox

Reviewers: the expiry logic in auth/tokens.ts is the part to scrutinize.
EOF
```

This keeps the squash commit clean while still giving reviewers everything they need.

## Creating the PR

Use a heredoc with `--body-file -` so the body stays plain and you avoid quote-escaping:

```bash
gh pr create \
  --title "feat(auth): add password reset flow" \
  --body-file - <<'EOF'
Let users who forget their password regain access without contacting
support.

- add forgot-password form with email validation
- generate and email single-use reset tokens
- rate-limit reset requests to curb abuse

Closes #123
EOF
```

Present the title, the body, and the exact command(s) for the user to confirm before running.

## Flags to suggest when relevant

| Flag | When |
|------|------|
| `--reviewer @user` | user names reviewers |
| `--assignee @me` | default, assign to self |
| `--label <name>` | when a label clearly applies |
| `--base <branch>` | target is not `main` |
| `--draft` | work is incomplete or wants early eyes |

For a draft, keep the same conventional title (no `WIP:` prefix — the draft state already signals that, and the title becomes the commit on merge):

```bash
gh pr create --draft --title "feat(auth): add password reset flow" --body-file - <<'EOF'
...
EOF
```

## Guardrails

- Never commit directly to `main`/`master`; branch first.
- One logical change per PR — a squash merge is one commit, so a PR should be one thing.
- Never force-push without explicit instruction.
- Confirm before pushing or opening the PR.
