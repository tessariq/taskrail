---
id: T-286-bind-verification-to-completion-and-adopt-legacy
title: Bind verification to completion and adopt legacy history
status: completed
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-285-add-stable-verification-identities-and-predecessor
updated_at: "2026-08-21T12:27:37Z"
completion_id: "e6275fffc2e3f59f88984f8f9d222028"
last_verification_id: "05de1c58cd45f96720789849073aa8e8"
last_verification_result: pass
last_verified_at: "2026-08-21T12:27:37Z"
last_verified_completion_id: "e6275fffc2e3f59f88984f8f9d222028"
---

# T-286-bind-verification-to-completion-and-adopt-legacy Bind verification to completion and adopt legacy history

## Description

Bind passing verification to the current completion and atomically adopt completion
identity for legacy completed tasks. This task owns completion-observation meaning
across all verification surfaces, not base ID generation or predecessor chaining.

## Acceptance

- A pass for a completed task with a non-empty `completion_id` records that exact
  value as `observed_completion_id`, task/state `last_verified_completion_id`, and
  matching report/note evidence; no other value is accepted.
- Pass before completion and every fail record null/none observed completion,
  remove any latest repository/task binding as applicable, and never change task
  status; T-241 owns the pass-before-complete warning.
- The first post-upgrade pass for a legacy completed task without `completion_id`
  atomically creates one and binds the same value on every surface; a fail does not
  adopt and any handled fault leaves the legacy shape unchanged.
- A current pass requires completed status and exact non-empty binding equality; a
  completed audit fail must directly follow the fresh bound pass from that run and
  retain completed delivered history.
- Validation distinguishes valid legacy completed history from a current bound pass
  and rejects partial, stale, unequal, fail-bound, or non-completed bindings.

## Verification Notes

- Run a lifecycle matrix for new completion, legacy completion, pass-before-
  complete, fail, stale pass, repeated complete, and completed audit fail.
- Compare task/state, command JSON, note, report, and artifact evidence for exact
  observed-ID agreement and direct audit predecessor linkage.
- Inject faults through legacy adoption and verify publication; assert no orphaned
  completion ID or partial binding survives.

## Implementation Notes

- 2026-08-21T12:27:26Z: Bound completed verification to exact completion evidence and preserved legacy adoption through migration.
- 2026-08-21T12:27:37Z: verification pass id 05de1c58cd45f96720789849073aa8e8 previous none completion e6275fffc2e3f59f88984f8f9d222028
