---
id: T-123-contributor-binary-resolution
title: Make the working-tree taskrail binary unmissable for contributors
status: completed
priority: low
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies: []
updated_at: "2026-07-28T13:30:23Z"
---

# T-123-contributor-binary-resolution Make the working-tree taskrail binary unmissable for contributors

## Description

Contributor-facing only: this is about developing Taskrail, not about adopters, whose
installed `taskrail` is correct by definition.

The committed skills resolve the binary as `${TASKRAIL:-taskrail}`. In this
repository the correct binary is the *working-tree* build, which only wins if
`mise run setup` has put `./bin` on `PATH`. In a non-interactive shell without
`mise activate` that never happens, so the bare fallback silently resolves to the
installed release instead.

Observed twice while implementing T-109 through T-118. Once loudly: `task new
--slug` failed with `unknown flag` because the on-PATH binary was the shipped
v0.3.0. Once quietly and expensively: `verify` ran against a `./bin` build made
before the fix in the same change set, and stamped four task files with the very
duplicate heading that change was fixing. Nothing errored; it was caught by reading
the files.

`task taskrail:check` does detect staleness, but it checks the binary on `PATH` and
its remedy (`task taskrail:install`) builds to `./bin` — a directory that may not be
on `PATH`. In that state the guard reports a condition its own advice cannot
resolve.

The guard misreports in a second way, on the same "the advice does not resolve what
was detected" theme (investigated 2026-07-27, `docs/binary-resolution-findings.md`
Finding 2). `internal/toolchain/cmd/freshcheck` byte-compares the binary, so the
*builder* changes the verdict, not just the source: `mise.toml` pins `go = "1.26"`,
which floats to 1.26.5, while a system `/usr/local/go` may be 1.26.0. Identical
source built under each produces different bytes, so any shell where the system Go
precedes mise's reports "stale" for a clean tree — and rerunning the advised `task
taskrail:install` in that same shell never converges. Both findings are the guard
sending a contributor somewhere that cannot fix what it detected, so they are
tracked here rather than split.

## Acceptance

- `task taskrail:install` fails loudly when its output directory is not reachable as
  `taskrail` and no `TASKRAIL` override is set, instead of succeeding and leaving the
  caller no better off. The failure names the two working fixes (`mise run setup`, or
  exporting `TASKRAIL`).
- The freshness guard's message distinguishes "the binary is stale" from "the binary
  you would run is not the one you just built", because the remedies differ.
- The guard no longer reports a clean tree as stale when `taskrail:install` and
  `taskrail:check` ran under different Go toolchains. Either pin `go` exactly in
  `mise.toml`, or have `freshcheck` recognize a toolchain difference (for example
  via `go version -m` build info) before falling back to the byte comparison —
  whichever it does, the message must name the toolchain as the variable so the
  remedy it prescribes is the one that resolves what it detected.
- The staleness window is closed for tracked-work commands in this repository: a
  state-writing command run against a binary older than the working tree is caught
  rather than trusted. A pre-command check, a documented wrapper, or hook wiring are
  all acceptable — the requirement is that a stale binary cannot silently write task
  or state files here.
- AGENTS.md documents the failure mode concretely, including the silent variant,
  since the loud one is self-explaining and the quiet one is not.
- No change to adopter-facing behavior: `${TASKRAIL:-taskrail}` stays correct for an
  installed binary, and nothing here ships in the packaged skills.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-28T13:30:17Z: verification pass
