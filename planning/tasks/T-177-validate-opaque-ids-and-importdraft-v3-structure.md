---
id: T-177-validate-opaque-ids-and-importdraft-v3-structure
title: Validate opaque IDs and ImportDraft v3 structure
status: todo
priority: high
spec_ref: specs/v0.6.0.md#portable-opaque-task-ids
dependencies:
    - T-176-classify-persisted-and-legacy-task-identities
updated_at: "2026-08-04T23:06:23Z"
---

# T-177-validate-opaque-ids-and-importdraft-v3-structure Validate opaque IDs and ImportDraft v3 structure

## Description

Add reusable opaque-ID and ImportDraft v3 structural validation needed by
migration and later writers, without enabling any layout-2 opaque write.

## Acceptance

- Opaque validation enforces exact grammar/length, folded generated and digest
  reservations, sibling/case/Unicode collisions, Windows devices, byte
  preservation, and allocation neutrality.
- Presence-aware TaskDraft id distinguishes absent from null/empty/whitespace
  and otherwise requires opaque validity.
- ImportDraft v1/v2 retain v0.5 meaning and reject id; strict v3 extends
  reviewed transactional v2 and old binaries reject schema 3 before tasks.
- Whole-draft validation plans explicit IDs, generated demand, dependency refs,
  and mixed-family collisions against supplied complete-ledger candidates
  without writes.
- APIs are pure validation/classification and cannot create, rename, import, or
  mutate tasks before layout 3.

## Verification Notes

- Map criteria to grammar/reserved/collision/Windows fixtures, null/absent id
  cases, v1/v2/v3 decoding, old-version rejection, and generated-exhaustion
  planning.
- Prove validators preserve exact opaque bytes and perform no filesystem write.

## Implementation Notes
