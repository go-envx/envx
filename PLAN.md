# Printer Migration: Adopt the Unified Output Layer Across All Actions

This document plans the migration of every action's rendering onto the shared `internal/printer` package (backed by `internal/style`), which now owns terminal capability detection, color and glyph styling, severity coloring, aligned tables, and JSON encoding. Each task is a single, independently reviewable pull request. The tasks share one transformation and have no ordering dependencies; the sequence runs from the highest-value, pattern-establishing changes to the trivial ones.

## Overview

### What already exists

`internal/printer` and `internal/style` are built, tested, and merged. `printer.New(printer.Options{Out, Err, Color})` returns a `*Printer` that auto-detects color per stream (enabled only for a terminal with `NO_COLOR` unset) unless `Color` overrides it. It exposes `LogMessage` (plain, stdout), `LogWarning` (`⚠  WARNING:`, stderr), `LogError` (`✗  ERROR:`, stderr), `WriteJSON` (indented, never colored, stdout), and `WriteTable` (bold header, severity-colored cells, ANSI-safe manual alignment). `style.Severity` maps to cyan (OK), yellow (warning), and red (error). Glyphs are emitted only when color is enabled; the label always carries severity in plain mode.

### The common transformation

Every action migration has the same shape:

- In `command.go`, build one printer — `printer.New(printer.Options{Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()})` — and pass it to `render`. `Color` stays `nil` (auto-detect) unless Task 9 lands.
- In `render.go`, replace the `Writer io.Writer` field on `renderParams` with a `*printer.Printer`, and rewrite the body to call the printer's semantic methods instead of `fmt.Fprintln`/`fmt.Fprintf`/`text/tabwriter`/a hand-rolled `json.Encoder`.
- Delete the now-dead local helpers (per-action `tabwriter` setup, ANSI constants, `isTerminal`, bespoke JSON encoders).
- Update `render_test.go` to construct a printer over `bytes.Buffer` sinks with `Color` forced (`&false` for deterministic plain assertions, `&true` to assert styling), using separate stdout and stderr buffers where an action writes to both.

### Cross-cutting decisions

- Severity banners move to stderr. `explain`'s banner becomes `LogError`/`LogWarning` (stderr) so stdout carries only the table. This improves piping (`explain … | grep` no longer sees the banner) and is an intentional behavior change to call out in that PR.
- `WriteJSON` replaces every action's private `json.NewEncoder(w); SetIndent; Encode` block; JSON output stays byte-for-byte uncolored.
- Actions keep their own `Format`/`Verbose`/`Reveal` flags; only the write mechanism changes.
- Prefer `str.Pluralize` for any count-and-noun summary (for example "Decrypted 1 secret" vs. "2 secrets") instead of a hard-coded `secret(s)` suffix. Apply it opportunistically as the remaining tasks touch each action's summary.
- Data outputs are not messages. `get` and `secrets get --reveal` print a raw value meant for capture in `$(…)`; that value stays a plain write and is never routed through a styling method.

### Out of scope

- `run` has no render layer — the child process owns stdout and stderr — so it is not migrated.
- No new severity levels, output formats, or table features beyond what the printer already provides.

## Tasks

- [x] **Task 1 — Migrate `explain` (flagship: table + banner + JSON + severity).** Replace `bannerLine` with `p.LogError`/`p.LogWarning` on stderr, passing only the message body (the printer owns the `ERROR:`/`WARNING:` label and glyph). Replace the `tabwriter` block with `printer.Table`: headers `KEY TYPE VALUE SOURCE STATUS` (plus `RESOLVED` under `--reveal`) and rows of `Cell`s whose STATUS cell carries a `style.Severity` mapped from `envmerge.Severity` via a small local `toStyleSeverity` helper (keeping `style` a dependency-free leaf). Replace `renderJSON` with `p.WriteJSON`. Delete the `text/tabwriter` import and `bannerLine`. Acceptance: masked and `--reveal` tables render, the JSON object is unchanged, the banner is on stderr, and `task envx:all` is green.

- [x] **Task 2 — Migrate `decrypt` and retire the duplicated color code.** Route the summary through `p.LogMessage` and the per-group skipped-key warnings through `p.LogWarning`. Delete the local `ansiYellow`/`ansiReset` constants, the local `isTerminal`, and the `Color`/`ErrWriter` fields threaded through `command.go` and `renderParams` — the printer now owns TTY detection and the stderr sink. Acceptance: warnings still go to stderr, colored on a TTY and plain when piped; the exit code is unchanged; tests assert via a printer over split buffers with forced color.

- [x] **Task 3 — Migrate `diff` (colored change table).** Replace the `tabwriter` rows with colored output: additions green `+`, removals red `-`, changes yellow `~`, following the conventional diff palette. Because green-for-added is not in the severity palette (OK is cyan) and the diff table is headerless, this PR extends the printer minimally: (a) skip the header row when `Table.Headers` is empty, and (b) let a cell carry an explicit color rather than only a severity (for example a `style.Color` enum on `Cell`). Replace the JSON encoder with `p.WriteJSON`. Acceptance: table and JSON shapes are unchanged aside from color, and a no-diff run still prints nothing.

- [x] **Task 4 — Migrate `encrypt`.** Convert the count-and-path summary, the verbose per-identity list, and the "No plaintext values to encrypt." line to `p.LogMessage`. Acceptance: output parity with today, and tests assert via a printer buffer.

- [ ] **Task 5 — Migrate `keypair` (generate, inspect, print, rotate).** Convert the four sibling summaries from `fmt.Fprintf` to `p.LogMessage`, one PR for the cohesive family. Acceptance: identical text, no private-key material in output, tests updated.

- [ ] **Task 6 — Migrate secrets confirmation summaries (`secrets set`, `secrets delete`).** Route each "… in: <path>" confirmation through `p.LogMessage`. Acceptance: output parity, tests updated.

- [ ] **Task 7 — Migrate config-writer summaries (`set`, `create`).** Route `set`'s "Set … in: <path>" and `create`'s scaffold summary through `p.LogMessage`; `create` currently prints from `command.go`, so pass its `summary()` string through the printer instead. Acceptance: output parity, tests updated.

- [ ] **Task 8 — Migrate value and presence outputs (`get`, `secrets get`).** Route the presence *message* (`secrets get` masked: "Secret … exists …") through `p.LogMessage`, but keep the raw *value* outputs (`get`, and `secrets get --reveal`) on a plain write so captured output stays byte-identical for scripting. `get` may otherwise be left unchanged. Acceptance: piped value output is unchanged, and the presence message matches today.

- [ ] **Task 9 — Global `--no-color` flag (optional).** Add a `schema` flag (`--no-color`; the printer already honors the `NO_COLOR` env var) and thread it into each command's `printer.Options.Color` (`false` when set, `nil` otherwise). Acceptance: `--no-color` forces plain output even on a TTY; default behavior is unchanged.
