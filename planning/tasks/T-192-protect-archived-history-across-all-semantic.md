---
id: T-192-protect-archived-history-across-all-semantic
title: Protect archived history across all semantic writers
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-storage-and-unified-ledger
dependencies:
    - T-185-upgrade-repositories-transactionally-to-layout-3
    - T-188-add-cancellation-provenance-and-dependency
    - T-189-bind-archive-eligibility-to-verification
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-179-resolve-stable-task-references-across-every
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-190-validate-unified-ledger-semantics-and-stable-read
updated_at: "2026-08-04T23:06:23Z"
---

# T-192-protect-archived-history-across-all-semantic Protect archived history across all semantic writers

## Description

Adopt complete-ledger allocation, resolution, safe representation, explicit live
write sets, and archive immutability across inherited semantic writers.

## Acceptance

- Ordinary writers acquire ownership before resolver/allocator/candidate reads,
  require layout 3, persist named live changes, and reject archive mutation;
  init migration retains its exception.
- Every generated entry point uses one locked allocator against
  archived/index-only claims; every target resolves before status/storage and
  new relationships use stable refs without aliases.
- A representation-sink registry proves every
  YAML/JSON/TSV/text/artifact/DOT/Mermaid writer uses safe helpers for
  scalar-looking opaque and control-bearing legacy identities.
- Existing live-task writers preserve `loop_policy` and `loop_reason` unchanged
  and never create separate policy state.
- New/imported/follow-up tasks remain live and omit task-local loop fields for
  implicit hold; follow-up to archived completed parent succeeds without parent
  mutation, while live/archived cancelled parents refuse.
- Archived sentinels remain byte/mode/mtime/path exact, including task-local loop
  fields, under every inherited writer.

## Verification Notes

- Map writer registry to pre-read lock,
  allocator/resolver/representation/write-set matrices, index-only maxima,
  exhaustion/concurrency, loop-field preservation, implicit-hold creation,
  follow-up parent classes, and sentinels.
- Mutation-test sink registration and race each inherited writer.

## Implementation Notes
