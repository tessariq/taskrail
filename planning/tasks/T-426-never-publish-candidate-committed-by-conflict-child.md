---
id: T-426-never-publish-candidate-committed-by-conflict-child
title: Never publish a candidate whose conflict child committed
status: todo
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T11:39:09Z"
---

# T-426-never-publish-candidate-committed-by-conflict-child Never publish a candidate whose conflict child committed

## Description

Bug. When a semantic replay conflict launches the integration child and that
child commits its own resolution, the candidate is reported `integration_failed`
and pending, but its bytes and its `completed` task status are still
fast-forwarded onto the source branch. The spec says failure leaves that
candidate unpublished.

Cause: after the conflict child, `integrateParallelCandidate`
(internal/taskrail/loop_parallel.go) checks only exit 0 and absent unmerged
paths, never that HEAD still equals the bound evolving head. The following
`git commit -C FETCH_HEAD` fails with nothing to commit, and
`parallelResetIntegration` resets to HEAD, which is now the child's commit. The
function also returns a nil child record, so the conflict child disappears from
`parallel.integration.children`; and a child that leaves unmerged paths is
recorded with outcome `pass`.

Repro: two tasks set the same line of shared.go to different values; the
conflict child runs `git checkout --theirs`, `git add`, `git commit`. Result:
`batch_partial`, T-002 pending, yet the source has the child's commit and T-002
`status: completed`. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- A conflict child that moves HEAD, or leaves unmerged paths, fails that candidate and the integration clone returns to the bound evolving head.
- None of a failed candidate's bytes or task status reach the published source branch.
- Every conflict child appears in `parallel.integration.children` with an outcome matching the candidate result.

## Verification Notes

- Integration test: conflict child that commits; assert the source HEAD tree lacks the candidate's bytes and the child record is present with outcome fail.

## Implementation Notes
