---
id: T-289-route-lifecycle-and-task-writers-through-local
title: Route lifecycle and task writers through local storage
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-282-protect-inherited-task-mutation-writers
    - T-286-bind-verification-to-completion-and-adopt-legacy
updated_at: "2026-08-08T14:23:08Z"
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
