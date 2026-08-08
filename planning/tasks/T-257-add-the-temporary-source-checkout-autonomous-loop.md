---
id: T-257-add-the-temporary-source-checkout-autonomous-loop
title: Add the temporary source-checkout autonomous loop
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-08T11:44:29Z"
---

# T-257-add-the-temporary-source-checkout-autonomous-loop Add the temporary source-checkout autonomous loop

## Description

Add one fail-closed, source-checkout-only autonomous runner for v0.5 development.
Keep its queue, prompt, tests, and local guidance explicit and inspectable, and
make every runtime artifact transient. This is temporary repository tooling, not
the product `taskrail loop` implementation.

## Acceptance

- `scripts/autonomous-loop/` contains the runner, complete v0.5 queue, lifecycle-
  complete prompt, sandbox test harness, and local agent guidance; committed files
  contain no credentials, sessions, logs, generated results, or provenance from
  outside this repository.
- Dry-run is genuinely read-only and reports the selected task only after checking
  main/remote alignment, clean Git state, queue integrity, active spec, task status,
  dependencies, planning validity, and working-tree binary freshness.
- Execution launches one fresh explicitly selected Claude or OpenCode process,
  prevents child Git delivery, verifies completed/pass or blocked/fail evidence,
  validates the final repository and binary, then creates exactly one direct-child
  commit and pushes `origin/main` itself. Any other outcome stops before another
  task without retry, reset, amend, force, or automatic repair.
- Queue validation rejects malformed/duplicate/missing/off-spec rows and dependency
  order drift. Every open v0.5 task appears once; operator and self-removal work are
  explicitly held, and the runner never mutates the queue.
- Shell tests use disposable repositories, fake agent commands, and local bare
  remotes to cover queue, dry-run, preflight, lifecycle, commit/push, and fail-stop
  behavior without launching a real agent or changing this repository.

## Verification Notes

- Run `bash -n scripts/autonomous-loop/run.sh scripts/autonomous-loop/test.sh` and
  `scripts/autonomous-loop/test.sh`.
- Run `scripts/autonomous-loop/run.sh --dry-run` only after the committed queue and
  binary are current; preserve its output as local operator evidence, not a
  committed runtime artifact.

## Implementation Notes

- Added the complete temporary control surface under `scripts/autonomous-loop/`.
  The runner validates the full dependency-ordered queue, repository/remote,
  current binary, lifecycle evidence, immutable prior reports, Git control state,
  and runner-owned commit/push delivery. Child Taskrail calls resolve through an
  external freshness-checking wrapper.
- The disposable-repository harness covers the real queue, dry-run purity,
  completed and blocked delivery, direct-child push, malformed queue state, dirty
  worktrees, stale binaries, blocked retry refusal, child failure, Git-control
  mutation, forged report JSON, and unqueued follow-up drift.
- The bounded review found weak Git-control/report checks, follow-up publication,
  blocked reselection, and underspecified v0.6 nested schemas. Those findings were
  fixed before lifecycle closure; no external provenance was retained.
- 2026-08-08T11:44:14Z: Added the disposable source-checkout loop, fail-closed queue and Git delivery checks, sandbox harness, mandatory retirement task, and approved v0.6 planning contracts.
- 2026-08-08T11:44:29Z: verification pass
