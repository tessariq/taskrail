---
id: T-445-give-sequential-loop-children-fresh-verification
title: Give sequential loop children fresh verification identities
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-443-gate-sequential-loop-runs-on-caller-owned-check
updated_at: "2026-11-01T01:06:43Z"
completion_id: "41912ed4c7180ec62fb94267566bf03c"
last_verification_id: "9f573107c2cb9b93b3558b0637f49d8d"
last_verification_result: pass
last_verified_at: "2026-11-01T01:06:43Z"
last_verified_completion_id: "41912ed4c7180ec62fb94267566bf03c"
---

# T-445-give-sequential-loop-children-fresh-verification Give sequential loop children fresh verification identities

## Description

Allow two sequential loop children to publish fresh, completion-bound passing
verification in one invocation. In an installed committed adopter with two
allowed tasks, a child that starts, completes, verifies pass and commits each
task currently succeeds once, then the second verify fails with
`generate verification id: delegation id is already present in preflight evidence`.
This reproduces on the T443 baseline without a check.

This follow-up owns verification identity freshness across sequential children,
not weaker identity validation or parallel check support. Preserve completion
binding, predecessor validation and delegated-writer authorization. T443's
observable second-child-launch control is not proof of two successful lifecycle
deliveries.

## Acceptance

- One `loop --max-iterations 2` invocation in a disposable committed adopter
  delivers both allowed tasks through real start, complete, verify-pass and Git
  commit operations, without a check or a second-task identity collision.
- The same two-task lifecycle succeeds with a passing caller-owned check. The
  check runs once on each delivered HEAD before any subsequent selection.
- Each task has a distinct fresh verification ID bound to its own completion
  ID. Its persisted task metadata, verification artifact and corresponding loop
  observations agree; any predecessor link retains its valid per-task meaning.
- Reusing a prior verification identity or artifact cannot satisfy a new child's
  fresh verification requirement. Incorrect completion bindings and invalid
  predecessor links remain rejected rather than accepted to enable continuation.
- Delegated writes still require valid authorization. Preserve the existing
  identity/replay refusal controls and do not bypass them in implementation or
  fixtures.

## Verification Notes

- Reproduce the baseline failure in a disposable installed adopter: create two
  eligible allowed tasks and a child that consumes the prompt, uses the staged
  TASKRAIL executable for start/complete/verify pass, and commits the resulting
  task/state/product bytes. Run with a budget of two and no check; retain exact
  argv, stderr and both persisted task states.
- Rerun that real CLI journey after the fix with and without a passing check.
  Require two child deliveries, both tasks completed with passing verification,
  two distinct verification IDs, valid completion bindings and real artifact
  contents. Check the final task/state summaries, attached HEAD ancestry, both
  delivery commits and clean worktree; launch markers alone are insufficient.
- In the checked variant, record each check's observed HEAD outside the worktree.
  Require exactly one check per delivered commit, in delivery order, and matching
  result-file evidence without changing task or verification bytes.
- Add a reuse/replay control that presents a previously used verification ID or
  stale passing artifact for the second child. Require refusal and no successful
  continuation. Separately mismatch the observed completion ID and predecessor
  link, and require the existing identity checks to reject each case.
- Retain delegated-writer authorization refusal tests. Run focused identity and
  sequential-loop tests plus the disposable CLI scenarios; record command output,
  result envelopes, artifact bindings and persisted Git/task/state observations.

## Implementation Notes

- 2026-11-01T01:06:31Z: Decoupled verification events from invocation-wide delegation identity while preserving fresh-ID, completion/predecessor and delegated authorization guards. Test-first two-delivery regression, full deterministic checks, bounded independent review and independent disposable CLI QA passed.
- 2026-11-01T01:06:43Z: verification pass id 9f573107c2cb9b93b3558b0637f49d8d previous none completion 41912ed4c7180ec62fb94267566bf03c
