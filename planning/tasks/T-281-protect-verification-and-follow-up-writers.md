---
id: T-281-protect-verification-and-follow-up-writers
title: Protect verification and follow-up writers transactionally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-233-protect-lifecycle-and-task-writers-transactionally
updated_at: "2026-08-08T14:23:08Z"
---

# T-281-protect-verification-and-follow-up-writers Protect verification and follow-up writers transactionally

## Description

Route `verify`, including `--create-followup`, through one normal transaction with
an exact artifact/task/state/follow-up write set. This task owns transactional
publication only; verification identity and completion-binding semantics remain
with T-285 and T-286.

## Acceptance

- Verify snapshots the selected task, state, relevant task ledger, and artifact
  destination before building and validating all candidates under one repository
  mutation lock.
- A verification without follow-ups publishes only its report/artifact, selected
  task, and state candidates; `--create-followup` additionally publishes exactly
  the requested fresh tasks and reprojects state from the complete candidate ledger.
- A delegated verify may create follow-ups only for its selected task and cannot
  mutate unrelated tasks or widen task-field capabilities.
- Any handled failure after the first publication restores every unchanged
  original and removes transaction-created paths; conflicts retain structured
  recovery evidence and never overwrite concurrent bytes.

## Verification Notes

- Exercise pass/fail verification with zero, one, and multiple follow-ups, using
  sentinel tasks to assert the complete consumed and published sets.
- Inject failures at artifact, task, follow-up, state, post-validation, and rollback
  boundaries and assert all-or-none observable bytes and common error snapshots.
- Run delegated negatives for wrong task, unapproved creation, and widened fields.

## Implementation Notes
