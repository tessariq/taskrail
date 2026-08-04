---
id: T-185-upgrade-repositories-transactionally-to-layout-3
title: Upgrade repositories transactionally to layout 3
status: todo
priority: high
spec_ref: specs/v0.6.0.md#layout-3-migration-and-compatibility
dependencies:
    - T-175-implement-arbitrary-width-generated-task-keys
    - T-176-classify-persisted-and-legacy-task-identities
    - T-177-validate-opaque-ids-and-importdraft-v3-structure
    - T-183-validate-cancellation-generation-and-archive
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-179-resolve-stable-task-references-across-every
    - T-181-detect-durable-physical-task-path-references
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-04T23:06:23Z"
---

# T-185-upgrade-repositories-transactionally-to-layout-3 Upgrade repositories transactionally to layout 3

## Description

Implement map-free config-only 2-to-3 and atomic 1-to-2-to-3 migration, debt
validation, archive adoption, and deterministic root discovery.

## Acceptance

- Apply owns the transaction before original-marker/repository resolution and
  holds it through candidate reads, publication, validation, rollback, or
  recovery.
- Direct 2-to-3 changes config only; multi-hop skills require consent, stage
  current bytes, and roll back marker/skills transactionally.
- Preview reports identity/portability/lifecycle/archive debt with exact
  warnings, zero-exit valid-with-warnings, non-null JSON, and zero writes.
- Existing archive follows absent/empty/adoptable/debt/blocker matrix and
  map-free apply changes no task bytes.
- Marker rollback rechecks all semantics; root discovery/lock choice, direct
  skill refresh, pre-write-only downgrade, and no hand-lowering are exact.

## Verification Notes

- Map criteria to lock-before-read, layouts/skills/old binary,
  archive/debt/warnings, rollback races, discovery/mismatch/cwd locks,
  downgrade/recovery, and previews.
- Persist clean, debt-heavy, adopted-archive, non-Git, and multi-hop reports.

## Implementation Notes
