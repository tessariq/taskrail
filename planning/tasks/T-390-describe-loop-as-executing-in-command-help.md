---
id: T-390-describe-loop-as-executing-in-command-help
title: Describe loop as executing in its command help
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-09-08T20:01:47Z"
completion_id: "9ab52d90866d4be05bda3b3e955e034f"
last_verification_id: "c2f75ad25a24b399e9c8f5fc981a8243"
last_verification_result: pass
last_verified_at: "2026-09-08T20:01:47Z"
last_verified_completion_id: "9ab52d90866d4be05bda3b3e955e034f"
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

- 2026-09-08T20:01:40Z: loop --help now describes execution: Long text states one external child process per selected task, names --dry-run as the preview-only mode that accepts no child command, calls out unsupported source-checkout execution, and splits the --parallel blast radius by delivery mode (clones plus committed-storage requirement always; integration commits and attached-branch fast-forward under --delivery local; caller-owned --review-adapter publish/open/merge under --delivery review). --parallel flag help reads as concurrently executed isolated tasks; usage line shows the required -- <command>. Help text only; Args, RunE, flag names, and defaults are byte-identical. Guarded by a command-surface test that binds any preview claim to --dry-run per sentence and rejects preview wording on execution flag lines.
- 2026-09-08T20:01:47Z: verification pass id c2f75ad25a24b399e9c8f5fc981a8243 previous none completion 9ab52d90866d4be05bda3b3e955e034f
