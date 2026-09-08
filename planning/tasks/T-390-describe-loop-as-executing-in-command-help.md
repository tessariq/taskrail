---
id: T-390-describe-loop-as-executing-in-command-help
title: Describe loop as executing in its command help
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-09-08T19:21:29Z"
---

# T-390-describe-loop-as-executing-in-command-help Describe loop as executing in its command help

## Description

`taskrail loop` is v0.5.0's flagship and highest-blast-radius command: it
selects deterministically, renders a prompt, and launches real external agent
processes, and under `--parallel` it clones the repository, runs workers, makes
integration commits, and fast-forwards the attached branch
(`internal/taskrail/loop_execute.go`, `internal/taskrail/loop_parallel.go`).

Its command help describes the opposite. `cmd/taskrail/loop.go:24` sets
`Short: "Preview deterministic unattended task execution"` and the command
registers no `Long`, so that single line is the entire description an operator
sees from `taskrail loop --help`. `cmd/taskrail/loop.go:68` compounds it:
`--parallel` is documented as "maximum isolated tasks to preview". That the
default is not a preview is settled by the separate opt-in `--dry-run` flag,
described as "report the selected task without launching a child".

`README.md` and `docs/loop.md` already describe the command correctly, so this is
help-surface drift alone — but the help is the surface an operator reaches before
any doc. Someone who reads it and runs `taskrail loop --parallel 3 -- <agent>`
expecting a preview instead launches three agent processes that clone the
repository, create commits, and publish a fast-forward.

## Acceptance

- `taskrail loop --help` states that the command executes: it launches an
  external child process per selected task, and names `--dry-run` as the
  preview-only mode.
- Flag help no longer describes execution as previewing; `--parallel` reads as a
  count of concurrently executed isolated tasks.
- The description reflects the parallel blast radius — repository clones, child
  processes, and the delivery/fast-forward boundary — without overstating it
  beyond the `specs/v0.5.0.md#cross-platform-autonomous-loop` contract.
- No behavior, flag name, default, or machine-result shape changes; this is help
  text only.

## Verification Notes

- Assert in a command-surface test that `loop`'s help does not describe the
  default invocation as a preview and that it names `--dry-run` as the
  preview mode, so the drift cannot silently return.
- Capture `taskrail loop --help` before and after as the observation.
- Re-run the full suite plus `task check:skills`; no packaged skill quotes the
  affected strings, so parity should be unaffected — confirm rather than assume.

## Implementation Notes
