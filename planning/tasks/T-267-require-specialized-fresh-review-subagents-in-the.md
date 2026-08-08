---
id: T-267-require-specialized-fresh-review-subagents-in-the
title: Require specialized fresh review subagents in the temporary loop
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-08T12:55:45Z"
---

# T-267-require-specialized-fresh-review-subagents-in-the Require specialized fresh review subagents in the temporary loop

## Description

Make the temporary autonomous-loop prompt require provider-neutral delegation
for simplification and correctness review. Prefer installed specialist
capabilities, fail closed when fresh delegation is unavailable, and apply an
explicit severity policy without weakening active-spec obligations.

## Acceptance

- Both Claude and OpenCode receive the same rendered instruction to inspect
  available skills and subagents, prefer specialist simplifier/reviewer
  capabilities, and otherwise use separate general-purpose fresh subagents with
  explicit lenses.
- Parent-context self-review does not satisfy either pass, each subagent reviews
  a frozen snapshot and returns findings or proposed changes for the parent to
  apply, and unavailable or failed fresh delegation blocks the task.
- High- and medium-severity current-scope findings are fixed. Low-severity
  observations remain report-only unless acceptance, specification, an
  invariant, or required test evidence makes the correction mandatory.
- The sandbox harness protects the rendered prompt contract for both supported
  backends, and local loop guidance records the invariant.

## Verification Notes

- Run `bash -n scripts/autonomous-loop/run.sh scripts/autonomous-loop/test.sh`,
  `scripts/autonomous-loop/test.sh`, and
  `scripts/autonomous-loop/run.sh --check-queue`.
- Run a read-only dry-run for each backend and the repository-wide Go,
  validation, skill-parity, and task-body checks.

## Implementation Notes

- 2026-08-08T12:55:34Z: Required provider-neutral fresh simplification and correctness subagents, specialist capability selection with general fallback, fail-closed review sequencing, severity dispositions, and shared Claude/OpenCode prompt-contract coverage.
- 2026-08-08T12:55:45Z: verification pass
