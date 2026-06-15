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
6. Present `gh pr create` command for user confirmation

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
## Summary
Brief description of what this PR accomplishes.

## Changes
- Bullet point list of key changes
- Derived from commit messages
- Grouped by logical area

## Testing
- How the changes were tested
- Any manual testing steps needed

## Related Issues
Closes #123
```

## Squash Merge Message

When the PR will be squash-merged, generate a comprehensive commit message:

```
feat(auth): add user authentication system (#45)

- Add login and registration forms
- Implement JWT token handling
- Add password reset flow
- Include rate limiting middleware

Co-authored-by: Name <email@example.com>
```

## Output Format

Present the generated PR with a ready-to-run command:

```
**PR Title:**
feat(auth): add user authentication

**PR Description:**
## Summary
Implements complete user authentication flow including login, registration, and password reset.

## Changes
- Add login form with email/password validation
- Add registration with email verification
- Implement JWT token storage and refresh
- Add password reset via email link

## Testing
- Unit tests added for auth utilities
- E2E tests cover login/logout flow
- Manual testing completed on Chrome and Firefox

Closes #42

**Run this command to create the PR:**
gh pr create --title "feat(auth): add user authentication" --body "## Summary
Implements complete user authentication flow including login, registration, and password reset.

## Changes
- Add login form with email/password validation
- Add registration with email verification
- Implement JWT token storage and refresh
- Add password reset via email link

## Testing
- Unit tests added for auth utilities
- E2E tests cover login/logout flow

Closes #42"
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
