---
title: Commands
description: The envx command surface.
sidebar:
  order: 1
---

| Command | Description |
| --- | --- |
| `envx get <project> <key>` | Resolve a project's environment and print one value. |
| `envx run <project> -- <command> [args...]` | Run a command with the merged environment injected. |
| `envx set <include-path> <key> <value>` | Write a key/value into a namespace's overlay file (dot notation supported for nested keys). |
| `envx explain <project> [key]` | Show each resolved value and the file it came from. |
| `envx diff <project> <env-a> <env-b>` | Compare a project's resolved environment across two environments. |

## Examples

```sh
# Get a value for a specific environment.
envx get api CREDENTIALS_PASSWORD --env staging

# Run with file values overriding existing OS env vars.
envx run api --overload -- npm start

# Require every overlay file in the chain to exist.
envx run api --strict -- ./deploy.sh

# Write a value into an overlay file (nested key, staging environment).
envx set env/database credentials.password s3cret --env staging

# Explain a single key and reveal its value.
envx explain api HOST --reveal

# Compare development against production as JSON.
envx diff api development production --output json
```
