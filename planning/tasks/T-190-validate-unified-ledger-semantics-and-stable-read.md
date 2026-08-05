---
id: T-190-validate-unified-ledger-semantics-and-stable-read
title: Validate unified ledger semantics and stable read snapshots
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-storage-and-unified-ledger
dependencies:
    - T-185-upgrade-repositories-transactionally-to-layout-3
    - T-183-validate-cancellation-generation-and-archive
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-179-resolve-stable-task-references-across-every
    - T-181-detect-durable-physical-task-path-references
    - T-184-recover-retained-semantic-transactions-explicitly
updated_at: "2026-08-04T23:06:23Z"
---

# T-190-validate-unified-ledger-semantics-and-stable-read Validate unified ledger semantics and stable read snapshots

## Description

Integrate semantic archived-state validity, migration-debt exceptions, path
drift, stable readers, and repair behavior over the raw combined loader.

## Acceptance

- Layout-3 validate rejects archived open or provenance-invalid content except
  exact adopted debt, enforces completed/cancelled dependency semantics, and
  rejects stale `AUTONOMY.tsv`; loop policy validates only from task-local
  `loop_policy` and `loop_reason` fields.
- Former-live/archive path scanning and non-Git incomplete-scope warnings
  integrate with validation using exact warning/violation exit behavior.
- Every read-only surface refuses active/recovery publication,
  snapshots/rechecks its complete consumed task bytes, including task-local loop
  fields, plus spec/state/config/prompt/index/attribute/scanned inputs, and
  outputs only stable bytes.
- Repair reprojects both roots but never moves tasks, chooses duplicate
  locations, clears recovery, or rewrites unchanged/archived bytes.
- Warning/remediation and reader behavior are deterministic across Git, non-Git,
  linked worktrees, and writer races.

## Verification Notes

- Map semantic archive/debt/dependency/path matrices, non-Git warnings, complete
  task-byte read-set registries, stale sidecar refusal, repair refusals, and
  archived sentinels.
- Race every reader with migration and each multi-file writer and prove no
  midpoint output.

## Implementation Notes
