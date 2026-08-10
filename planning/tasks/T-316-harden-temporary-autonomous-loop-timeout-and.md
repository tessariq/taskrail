---
id: T-316-harden-temporary-autonomous-loop-timeout-and
title: Harden temporary autonomous loop timeout and recovery
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-10T18:56:53Z"
---

# T-316-harden-temporary-autonomous-loop-timeout-and Harden temporary autonomous loop timeout and recovery

## Description

Bound the temporary source-checkout loop's external agent execution, preserve
enough evidence to resume parent-owned delivery explicitly after interruption,
and let verification-created follow-ups enter the reviewed queue only as held
work. Tighten the shared prompt where EpochFC's evidence checkpoints improve
instruction following without importing its child-owned Git delivery or runnable
follow-up policy.

## Acceptance

- A1. Claude and OpenCode attempts have a configurable positive wall-clock
  timeout with a two-hour default; timeout or interruption terminates the
  observable process group, never retries, never selects again, and leaves
  durable runner diagnostics.
- A2. A valid completed/pass or blocked/fail child outcome is snapshotted before
  Git delivery in a private XDG-state recovery bundle; explicit delivery resume
  rechecks repository, Git, queue, report, lifecycle, binary, message, and
  worktree identities before one commit/push, while stale or incomplete bundles
  write nothing.
- A3. A delegated child may create one genuinely separate follow-up only through
  its fresh verification report. The parent appends that exact task as
  `hold-operator`, records the advisory recommendation, validates the queue, and
  cannot execute it in the same invocation; every other task or loop-control
  mutation is refused.
- A4. The shared prompt requires concise evidence checkpoints, observable outcome
  and invariant framing, four exact review dispositions, and mutation proof for
  weak-test findings while retaining parent-owned Git delivery and bounded fresh
  reviews.
- A5. Disposable harness cases cover normal delivery, terminal and in-progress
  hangs, timeout/interruption cleanup, follow-up acceptance/refusal, recovery and
  tamper rejection, no retry, frozen selection, and byte-identical Claude/OpenCode
  prompts.

## Verification Notes

- Run shell syntax and the autonomous-loop harness, Taskrail queue/state checks,
  repository Go checks, and a sandboxed manual timeout plus delivery-resume flow.

## Implementation Notes

- 2026-08-10T18:56:30Z: Added bounded no-retry agent execution, private bundle-bound delivery resume, report-authorized held follow-ups, evidence-driven prompt checkpoints, and adversarial loop coverage.
- 2026-08-10T18:56:53Z: verification pass
