---
name: pull-request
description: Creates pull requests with well-structured titles and descriptions. Analyzes commit history to generate PR content, suggests reviewers, and provides ready-to-run gh CLI commands. Use when opening a PR or preparing a feature branch for merge.
---

You are an expert at creating clear, comprehensive pull requests that facilitate code review.

## Workflow

1. Run `git branch --show-current` to get the current branch name
2. Run `git log main..HEAD --oneline` to see commits in this branch (adjust base branch if needed)
3. Run `git diff main..HEAD --stat` to see files changed
4. Optionally run `git diff main..HEAD` for detailed changes
5. Generate PR title and description
6. Run `gh pr create` command to create the PR
7. Run `gh pr comment <pr-number> --body` command to add comments or request reviews

## PR Title Format

Use conventional commit style for the title:

```
<type>(<scope>): <description>
```

Derive from the branch name and commit history:
- `feature/user-auth` → `feat(auth): add user authentication`
- `fix/login-validation` → `fix(login): correct validation logic`
- `chore/update-deps` → `chore(deps): update dependencies`

## PR Description Template

```markdown
Brief description of what this PR accomplishes.

- Bullet point list of key changes
- Derived from commit messages and diff stats
- Grouped by logical area
- Include key unit tests or integration tests added

Closes #123 (if applicable)
```

## Output Format

All PRs are squash-merged. Treat the generated PR title and description as the squash commit subject and body. Do not generate a separate squash merge message unless explicitly requested.

Present the generated PR with a ready-to-run command:

```
**PR Title:**
feat(auth): add user authentication

**PR Description:**
## Summary
Implements complete user authentication flow including login, registration, and password reset.

- Add login form with email/password validation
- Add registration with email verification
- Implement JWT token storage and refresh
- Add password reset via email link

Closes #42
```

**Run this command to create the PR:**

```
gh pr create --title "feat(auth): add user authentication" --body "## Summary
Implements complete user authentication flow including login, registration, and password reset.

- Add login form with email/password validation
- Add registration with email verification
- Implement JWT token storage and refresh
- Add password reset via email link

Closes #42"
```

## Additional Comments

After generating the PR, it may be helpful to provide additional context or notes for reviewers that do not belong in the squash merge commit message. Use the following template:

```markdown
- Bullet point list of areas that may require special attention
- List any manual testing steps completed or edge cases that were considered
- Focus should be on improving the overall review process
- Request specific reviewers if needed
```

**Run this command to add comments to the PR:**

```
gh pr comment <pr-number> --body "Review focus for the authentication change:
- Manually tested registration, email verification, login, logout, token refresh, and password reset, including duplicate emails, invalid credentials, rate limiting, and expired or reused links
- Found and fixed an edge case where password reset left existing sessions active; verified that old sessions and the old password were rejected afterward
- Reviewed password hashing, credential logging, token expiry and revocation, authorization, and cross-user access checks
- Did not manually verify CSRF behavior or cookie flags; automated tests covered those cases"
```

## Additional Options

Suggest relevant flags based on context:

| Flag | When to Suggest |
|------|-----------------|
| `--reviewer @username` | When user mentions specific reviewers |
| `--assignee @me` | Default, assign to self |
| `--label bug` | When PR fixes a bug |
| `--label enhancement` | When PR adds a feature |
| `--draft` | When work is not yet complete |
| `--base develop` | When target branch is not main |

## Draft PRs

For work-in-progress, suggest creating a draft:

```bash
gh pr create --draft --title "WIP: feat(auth): user authentication" --body "..."
```
