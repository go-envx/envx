---
title: Settings Reference
description: Every setting you can place in envx.yaml, with its flag, env var, and default.
sidebar:
  order: 2
---

Settings control how envx loads your files and renders the resulting keys. Every setting can be declared under a top-level `settings` block (global) or a project's `settings` block, and each can also be supplied as a CLI flag or an `ENVX_*` environment variable. See [Settings Resolution](/configuration/settings-resolution/) for the precedence rules, and the [Examples](/configuration/examples/) page for a worked example of each setting.

## env

The environment to resolve. It selects which `<namespace>.<environment>.yaml` overlays are applied on top of each namespace's base file.

- **Flag:** `--env`, `-E`
- **Environment variable:** `ENVX_ENV`
- **Manifest key:** `env`
- **Default:** the first declared environment

## strict

When `true`, every environment overlay file in a project's namespace chain must exist. A missing overlay becomes an error instead of being silently skipped.

- **Flag:** `--strict`
- **Environment variable:** `ENVX_STRICT`
- **Manifest key:** `strict`
- **Default:** `false`

## prefix

A string prepended to every resolved key. It is uppercased and joined with an underscore, so a prefix of `app` turns `HOST` into `APP_HOST`.

- **Flag:** `--prefix`
- **Environment variable:** `ENVX_PREFIX`
- **Manifest key:** `prefix`
- **Default:** `""`

## suffix

A string appended to every resolved key. It is uppercased and joined with an underscore, so a suffix of `v2` turns `HOST` into `HOST_V2`.

- **Flag:** `--suffix`
- **Environment variable:** `ENVX_SUFFIX`
- **Manifest key:** `suffix`
- **Default:** `""`

## namespace_prefix

When `true`, each key is prefixed with the name of the namespace it came from, keeping values from different namespaces distinct.

- **Flag:** `--namespace-prefix`
- **Environment variable:** `ENVX_NAMESPACE_PREFIX`
- **Manifest key:** `namespace_prefix`
- **Default:** `false`

## delimiter

The string used to join a list-valued leaf into a single environment variable. A YAML sequence such as `[a, b, c]` becomes `a,b,c` by default.

- **Flag:** `--delimiter`
- **Environment variable:** `ENVX_DELIMITER`
- **Manifest key:** `delimiter`
- **Default:** `","`

## overload

When `true`, values resolved from your files take precedence over existing OS environment variables. When `false`, an existing OS variable of the same name is left untouched.

- **Flag:** `--overload`
- **Environment variable:** `ENVX_OVERLOAD`
- **Manifest key:** `overload`
- **Default:** `false`
