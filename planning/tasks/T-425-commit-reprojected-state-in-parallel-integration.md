---
id: T-425-commit-reprojected-state-in-parallel-integration
title: Commit reprojected STATE.md in parallel integration commits
status: todo
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T11:39:09Z"
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
