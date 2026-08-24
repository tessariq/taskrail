---
id: T-358-prevent-read-only-git-probes-from-refreshing-index
title: Prevent read-only Git probes from refreshing the index
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-333-preview-deterministic-parallel-clone-batches
updated_at: "2026-08-24T07:56:32Z"
completion_id: "224e829e19e7c9fbb49ad3dca94ed233"
last_verification_id: "f975a02f8b21195561f29981686d7a0b"
last_verification_result: pass
last_verified_at: "2026-08-24T07:56:32Z"
last_verified_completion_id: "224e829e19e7c9fbb49ad3dca94ed233"
---

# T-358-prevent-read-only-git-probes-from-refreshing-index Prevent read-only Git probes from refreshing the index

## Description

Keep Taskrail's read-only Git probes from refreshing `.git/index` or taking
optional Git locks. The parallel dry-run side-effect assertion failed on the
exact readiness head when `git status` refreshed index metadata, even though no
managed or worktree bytes changed. Bind shared read-only Git commands to Git's
documented optional-lock suppression rather than weakening repository snapshots.

## Acceptance

- Shared read-only Git probes set `GIT_OPTIONAL_LOCKS=0` for the child command
  while preserving the caller's complete environment and without changing
  writer-owned Git commands.
- A fixture with deliberately stale tracked-file stat metadata proves `git
  status` reports the correct clean result without changing exact index bytes or
  creating an index lock.
- Sequential and parallel loop dry-runs remain byte-side-effect-free, and
  existing Git inspection, delivery validation, prompt-path, local-mode, and
  review-publication behavior remains unchanged.

## Verification Notes

- Start with a failing focused test that changes a tracked file's mtime without
  changing content, snapshots `.git/index`, invokes the shared read-only status
  path, and compares the index and lock state.
- Run focused loop preflight and Git consumers, repeated dry-run tests, full and
  race tests, vet, build, Taskrail validation, task-body/queue checks, and the
  exact-head CI matrix.

## Implementation Notes

- 2026-08-24T07:56:23Z: Set GIT_OPTIONAL_LOCKS=0 on shared read-only Git probes and added a stale-index regression; focused repetitions and full tests pass, while the separate race timing fixture is tracked as T-359.
- 2026-08-24T07:56:32Z: verification pass id f975a02f8b21195561f29981686d7a0b previous none completion 224e829e19e7c9fbb49ad3dca94ed233
