---
id: T-234-protect-repository-and-planning-writers
title: Protect init and retrofit writers transactionally
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-277-add-durable-transaction-journals-and-recovery
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-19T23:19:29Z"
---

# T-234-protect-repository-and-planning-writers Protect init and retrofit writers transactionally

## Description

Route fresh/adopted `init` and `retrofit` apply paths through the shared transaction
substrate while preserving preview purity and each layout operation's required
durability. Repair/spec and ImportDraft v1 publication are separate slices.

## Acceptance

- Fresh/adopted init and retrofit apply acquire the correct repository lock,
  snapshot every inspected destination/source, validate the complete candidate,
  and publish only the exact reported config/spec/state/task/note/skill set.
- Operations that require durable recovery record originals, candidates, and phase
  before semantic publication; normal cases use the shared normal transaction and
  both classes expose common conflict/recovery details.
- Handled failures restore every unchanged original and preserve external edits;
  successful results describe the bytes actually published.
- Preview and prompt-emission paths remain write-free, create no lock/transaction
  artifacts, and return a rechecked stable candidate snapshot.

## Verification Notes

- Run fresh, adopted, retrofit, and refusal fixtures with exact reported-versus-
  actual write sets and sentinels around every unrelated destination.
- Inject failures and concurrent edits at each publication/rollback phase and
  assert recovery records and common machine-error snapshots.
- Compare filesystem, Git index, and lock-root digests around every preview path.

## Implementation Notes

- 2026-08-19T23:19:17Z: Published fresh/adopted init and retrofit scaffolds through locked normal transactions with stable previews, pre-publication validation, conflict-safe rollback, and backup-first skill refresh.
- 2026-08-19T23:19:29Z: verification pass
