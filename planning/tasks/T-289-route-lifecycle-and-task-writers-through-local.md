---
id: T-289-route-lifecycle-and-task-writers-through-local
title: Route lifecycle and task writers through local storage
status: completed
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-282-protect-inherited-task-mutation-writers
    - T-286-bind-verification-to-completion-and-adopt-legacy
updated_at: "2026-08-21T14:13:07Z"
completion_id: "b9f3b8a641c38e8a6fba7c490b42bd56"
last_verification_id: "58dc59723b90b83e30b1189c070492c9"
last_verification_result: pass
last_verified_at: "2026-08-21T14:13:07Z"
last_verified_completion_id: "b9f3b8a641c38e8a6fba7c490b42bd56"
---

# T-289-route-lifecycle-and-task-writers-through-local Route lifecycle and task writers through local storage

## Description

Route inherited state-selection, lifecycle, verification, and task mutation writers
through the active storage context. This task owns local physical publication for
those already-defined commands; implicit bootstrap policy and integration remain
with T-245 and the final parity gate.

## Acceptance

- `next`, lifecycle commands, `verify` with follow-ups, `task new`, rename, repoint,
  and dependency mutations consume local semantic bytes through logical identities
  and publish only beneath the ignored local storage context.
- Each command preserves its committed-mode lifecycle, exact-ID, completion,
  verification-chain, validation, transaction, and machine-result semantics; only
  physical storage and delivery classification differ.
- Local rename and other publications use filesystem transaction operations, never
  stage ignored files, and leave the Git index and ordinary status unchanged.
- Decoy committed semantic files are never read or changed while local mode is
  active; mixed or unsupported contexts refuse without fallback or split state.
- No command implements a second local scaffold path; this slice operates against
  an already initialized active context and leaves implicit initialization to T-245.

## Verification Notes

- Run paired committed/local command scenarios and compare logical results and
  post-state for selection, lifecycle, verify/follow-up, create, rename, repoint,
  and dependency edits.
- Assert exact local physical writes, unchanged decoy committed files, clean index/
  status, and transaction rollback at every multi-file fault point.
- Run the matrix from explicit local initialization; T-291 owns the combined
  implicit-bootstrap and inherited-writer parity evidence.

## Implementation Notes

- 2026-08-21T14:12:59Z: Routed lifecycle and task writers through validated local storage.
- 2026-08-21T14:13:07Z: verification pass id 58dc59723b90b83e30b1189c070492c9 previous none completion b9f3b8a641c38e8a6fba7c490b42bd56
