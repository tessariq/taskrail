---
id: T-423-report-clean-failed-loop-child-as-child-failed
title: Report a clean failed loop child as child_failed
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T11:36:49Z"
---

# T-423-report-clean-failed-loop-child-as-child-failed Report a clean failed loop child as child_failed

## Description

Bug. A loop child that exits non-zero, or is stopped by a handled SIGINT,
without committing anything is reported as `invalid_postflight` with
`delivery_commit_shape` instead of `child_failed`. The outcome precedence
makes a non-zero or signalled child `child_failed`, the quota section calls
that the ordinary case for a generic non-zero child, and a handled interrupt
must publish `child_failed`. The spec now states it directly: a non-zero or
signalled child that leaves HEAD unchanged delivered nothing, so delivery
commit-shape requirements do not apply to it.

Cause: `validateLoopDelivery` (internal/taskrail/loop_delivery.go) skips the
commit-shape check for a failed child only when the tree is dirty and HEAD is
unchanged. A clean failed child with an unchanged HEAD falls through to the
committed/local commit-shape check. `TestLoopExecuteStopsAfterFailedChild`
currently asserts the wrong outcome.

Repro in a disposable repo with one allowed task:
`taskrail loop --result-file /tmp/out/a.json -- sh -c 'cat >/dev/null; exit 1'`
gives `outcome: invalid_postflight`, `mutation_violations: [delivery_commit_shape]`,
`child.exit_code: 1`. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- A child that exits non-zero with a clean tree and unchanged HEAD yields per-child and terminal outcome `child_failed`, a non-zero exit, and empty `mutation_violations`.
- A handled SIGINT/SIGTERM during a child publishes `child_failed` in the result file.
- A failed child whose writes violate integrity (for example a moved sibling ref) still yields `invalid_postflight`.
- `TestLoopExecuteStopsAfterFailedChild` asserts `child_failed`.

## Verification Notes

- Unit/integration: run the existing nonzero helper child through `LoopExecute` and assert the outcome and empty mutation violations.
- Manual: rerun the CLI repro and an interrupted run in a temp repo and inspect the result file.

## Implementation Notes
