---
id: T-158-bind-completion-and-verification-with-stable
title: Persist stable completion identities legally
status: completed
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-229-canonicalize-v0-5-lifecycle-and-task-identities
    - T-233-protect-lifecycle-and-task-writers-transactionally
updated_at: "2026-08-19T23:06:42Z"
completion_id: "e7c6426f8750d03c3d9d18c10b545296"
---

# T-158-bind-completion-and-verification-with-stable Persist stable completion identities legally

## Description

Implement completion identity creation and the closed legal persisted metadata
shapes consumed by lifecycle writers and validation. Verification IDs, chains,
artifacts, completion binding, legacy adoption, and advisory warnings are separate
dependent outcomes.

## Acceptance

- Each successful `complete` creates a preflight-absent random lower-case 32-hex
  `completion_id`, returns it in the exact complete result, and persists it on the
  selected completed task in the same transaction.
- Complete clears every prior task-level verification field while preserving the
  repository-level latest verification tuple and unrelated task history exactly.
- Shared strict decoding and validation accept only the closed lifecycle metadata
  combinations: optional keys are omitted rather than null/empty, IDs have the
  required form, and partial or status-incompatible completion/verification shapes
  are invalid.
- `start`, `block`, `unblock`, and task release preserve any existing completion
  and verification fields exactly; repeating complete creates a new completion ID
  and again clears task-level verification evidence.
- This task exposes completion primitives used by later verification binding but
  does not create verification IDs, reports, notes, artifact names, or adoption.

## Verification Notes

- Use deterministic ID injection to assert complete JSON/task persistence, fresh ID
  replacement, task-level verification clearing, and repository-history retention.
- Run a table over every allowed and forbidden persisted metadata combination,
  including omitted/null/empty fields and all lifecycle statuses.
- Snapshot tasks/state across start, block, unblock, release, and repeated complete;
  inject complete publication failures to prove no ID is partially persisted.

## Implementation Notes

- 2026-08-19T23:06:41Z: Implemented atomic stable completion identities, strict lifecycle metadata validation, preservation semantics, and regression coverage.
- 2026-08-19T23:06:42Z: verification pass
