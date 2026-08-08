---
id: T-195-report-unified-ledger-storage-without-semantic
title: Report unified ledger storage without semantic drift
status: todo
priority: medium
spec_ref: specs/v0.6.0.md#archive-storage-and-unified-ledger
dependencies:
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-190-validate-unified-ledger-semantics-and-stable-read
    - T-191-add-stable-task-inspection-and-filtered-inventory
    - T-194-add-explicit-archive-and-restore-commands
    - T-167-add-active-spec-scoped-statistics
updated_at: "2026-08-08T08:40:49Z"
---

# T-195-report-unified-ledger-storage-without-semantic Report unified ledger storage without semantic drift

## Description

Update status, stats, coverage, gaps, graphs, repair, allocation, and
verification history to consume the combined ledger while exposing storage
without lifecycle drift.

## Acceptance

- Status/stats JSON adds exactly `task_storage:{live,archived}` and equivalent
  text; status retains inherited `storage` mode/root/artifacts context unchanged,
  and the task-storage sum matches total while inherited
  done/cancelled/distribution semantics remain.
- Coverage, gaps, spec linkage, verification history, dependencies, chains, and
  active-spec projections are invariant under pure archive moves.
- DOT/Mermaid retain archived nodes, label storage, use injective keys, and
  preserve cross-storage edges without aliases.
- Repair/count projection includes both roots but pure move does not rewrite
  STATE/path lists.
- Omitted/default outputs retain documented compatibility except specified
  additive storage fields.

## Verification Notes

- Map criteria to exact before/after text/JSON/graph goldens, mixed cohorts,
  active filters, zero roots, and collision labels.
- Confirm report commands remain read-only under stable snapshot races.

## Implementation Notes
