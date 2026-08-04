---
id: T-184-recover-retained-semantic-transactions-explicitly
title: Recover retained semantic transactions explicitly
status: todo
priority: high
spec_ref: specs/v0.6.0.md#atomic-git-aware-moves-and-recovery
dependencies:
    - T-180-make-semantic-publication-durably-transactional
    - T-182-define-exact-v0-6-machine-result-schemas
updated_at: "2026-08-04T23:06:23Z"
---

# T-184-recover-retained-semantic-transactions-explicitly Recover retained semantic transactions explicitly

## Description

Add inspect, preview, restore-original, and accept-candidate recovery over
retained durable transactions, including crash safety of recovery itself.

## Acceptance

- Actions are mutually exclusive, refuse a live owner, and default to read-only
  inspection; preview writes nothing.
- Restore accepts each component only at recorded original/candidate value
  including expected absence, restores candidate-valued components, leaves
  originals, validates all-original, then clears.
- Accept requires the complete candidate exactly, validates it, clears recovery
  without semantic writes, and never accepts a partial candidate.
- Missing/unexpected/external/invalid state preserves current data and backups
  with bounded diagnostics; reset/checkout/inference are never used.
- Recovery actions have their own durable phases and remain retryable after
  death during every write, sync, validation, and cleanup boundary.

## Verification Notes

- Map criteria to live owner, flag conflicts, every mixture, unexpected
  absence/presence, invalid outcomes, and exact inspect/restore/accept
  observations.
- Kill each recovery action after every durable boundary and prove safe
  deterministic retry.

## Implementation Notes
