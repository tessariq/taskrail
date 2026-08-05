---
id: T-219-prepare-inherited-writers-for-layout-3
title: Prepare inherited writers for layout 3
status: todo
priority: high
spec_ref: specs/v0.6.0.md#layout-3-migration-and-compatibility
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-179-resolve-stable-task-references-across-every
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-184-recover-retained-semantic-transactions-explicitly
updated_at: "2026-08-05T20:24:40Z"
---

# T-219-prepare-inherited-writers-for-layout-3 Prepare inherited writers for layout 3

## Description

Prepare every inherited semantic writer for layout-3 combined-ledger identity and
archive immutability before migration can make layout 3 active.

## Acceptance

- Behind the layout-3 guard, inherited lifecycle/task/spec/import/review writers use the combined resolver/allocator and explicit live-only write sets.
- Every inherited writer rejects archived mutation and includes archive/index-only claims in identity allocation and relationship resolution before T-185 can publish layout 3.
- Layout-1/2 behavior remains unchanged until migration; unsupported layouts still refuse semantic writes and no archive root is created by preparation alone.
- Migration depends on this readiness, eliminating any accepted-layout window with live-only allocation or mutable archived history. T-192 retains exhaustive sink/write-set verification.
- Existing task-local loop fields, opaque/control-safe rendering, transaction ownership, and recovery behavior remain preserved.

## Verification Notes

- Register every inherited writer and race it against live/archive sentinels, index-only identities, ambiguous references, and allocation above archived maxima.
- Prove layout-2 behavior compatibility, layout-3 readiness before T-185, archived byte/mode/mtime preservation, and no migration side effects.

## Implementation Notes
