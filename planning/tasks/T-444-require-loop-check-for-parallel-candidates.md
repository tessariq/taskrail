---
id: T-444-require-loop-check-for-parallel-candidates
title: Require the loop check for parallel candidates
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-443-gate-sequential-loop-runs-on-caller-owned-check
updated_at: "2026-11-01T00:37:22Z"
completion_id: "5b16de81853cbbbfbc63bee76d65b7e3"
last_verification_id: "af670805ec75fd60ccd2fc8d53ab8491"
last_verification_result: pass
last_verified_at: "2026-11-01T00:37:22Z"
last_verified_completion_id: "5b16de81853cbbbfbc63bee76d65b7e3"
---

# T-444-require-loop-check-for-parallel-candidates Require the loop check for parallel candidates

## Description

Implement the parallel half of the `loop --check` gate in Parallel Isolated
Clone Batches. Today a clean replay is committed without any check, and the
final "aggregate gate" is the caller's agent command exiting zero. The review
reproduced a `batch_pass` that published a tree which does not build (one task
renamed `Old()`, another added a caller of it).

The spec now requires `--check` whenever the effective parallel width is above
one, runs it on each candidate's integration commit, and runs it again on the
final integration head after the aggregate child, before the fast-forward.

## Acceptance

- Executing with an effective width above one and no `--check` is `invalid_arguments` before any workspace or child is created, and the matching dry-run report is `invalid` with that reason.
- Each candidate's integration commit is checked in the integration clone; a failing or mutating check makes that candidate `integration_failed`, resets the clone to the prior evolving head, and does not block later independent candidates.
- Publication requires a passing aggregate child followed by a passing check on the same bound head; otherwise no fast-forward happens and retention follows the failure policy.
- Per-candidate and final check evidence appears in the parallel diagnostic, using the companion shape defined in T-443.
- The semantic-conflict scenario (rename plus new caller in another file) with a build check no longer publishes a non-building tree.

## Verification Notes

- Integration test reproducing the rename/caller scenario with a check that runs a build.
- Smoke test for the missing-check refusal in parallel dry-run and execution.

## Implementation Notes

- 2026-11-01T00:37:14Z: Implemented required local parallel candidate and final same-head checks with retained evidence, safe rollback and mutation refusal, plus preflight refusal of unsafe checked review delivery. Bounded independent review, full deterministic checks and frozen-candidate disposable integrated QA passed.
- 2026-11-01T00:37:22Z: verification pass id af670805ec75fd60ccd2fc8d53ab8491 previous none completion 5b16de81853cbbbfbc63bee76d65b7e3
