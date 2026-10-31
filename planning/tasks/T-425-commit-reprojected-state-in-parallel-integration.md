---
id: T-425-commit-reprojected-state-in-parallel-integration
title: Commit reprojected STATE.md in parallel integration commits
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T15:07:56Z"
completion_id: "771a86112304f61703f233d7fdc624d9"
last_verification_id: "c15e8aed824c7f132a0671a9c8ee5cf0"
last_verification_result: pass
last_verified_at: "2026-10-31T15:07:56Z"
last_verified_completion_id: "771a86112304f61703f233d7fdc624d9"
---

# T-425-commit-reprojected-state-in-parallel-integration Commit reprojected STATE.md in parallel integration commits

## Description

Bug. A successful parallel batch (`batch_pass`) publishes a `planning/STATE.md`
whose task counts are stale. The spec requires each per-task integration commit
to contain "current reprojected state".

Cause: `integrateParallelCandidate` (internal/taskrail/loop_parallel.go) runs
`Repair(Apply)` after `git cherry-pick --no-commit FETCH_HEAD`, then commits with
`git commit -C FETCH_HEAD` without staging the rewritten STATE.md. The
reprojection stays unstaged in the integration clone and is lost.

Repro: two allowed independent tasks, `--parallel 2 --max-iterations 2`, workers
that complete and commit. After `batch_pass`, `taskrail repair` on the source
reports `todo: 1 -> 0`, `completed: 1 -> 2`. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- After `batch_pass`, `taskrail repair` (dry run) on the source reports no changes.
- Every integration commit whose reprojection changed STATE.md includes it, and the integration clone is clean after each commit.
- `taskrail validate` stays valid on the published source.

## Verification Notes

- Integration test: two-worker batch, then a dry-run `Repair` on the source returns an empty BodyDiff.

## Implementation Notes

- 2026-10-31T15:07:46Z: Stage the CLI-reprojected state literally in each integration commit. Real two-worker and metacharacter-path regressions pass; independent integrated QA confirms cumulative counts, clean integration commits, and publication/refusal controls. Scoped review finding fixed and independently closed.
- 2026-10-31T15:07:56Z: verification pass id c15e8aed824c7f132a0671a9c8ee5cf0 previous none completion 771a86112304f61703f233d7fdc624d9
