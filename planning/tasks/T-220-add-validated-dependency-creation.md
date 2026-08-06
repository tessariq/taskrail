---
id: T-220-add-validated-dependency-creation
title: Upgrade dependency editing to stable references
status: todo
priority: high
spec_ref: specs/v0.6.0.md#cancellation-transition-and-provenance
dependencies:
    - T-179-resolve-stable-task-references-across-every
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-185-upgrade-repositories-transactionally-to-layout-3
updated_at: "2026-08-05T20:24:45Z"
---

# T-220-add-validated-dependency-creation Upgrade dependency editing to stable references

## Description

Upgrade inherited exact-ID dependency add/remove to the unified stable-reference
resolver and live-plus-archive ledger without changing their one-edge semantics.

## Acceptance

- Both inherited commands resolve target/dependency through the unified ledger,
  support dry-run/envelope-v2 JSON, and persist or remove one semantic stable edge.
- Duplicate, self, cycle, missing, ambiguous, cancelled-dependency, archived-target, alias, and recovery-pending cases refuse before writes with stable codes.
- Candidate validation uses complete-ledger semantics, preserves existing dependency order and task-local loop fields, reprojects STATE, and publishes through the durable transaction protocol.
- Add/remove share exact result/error types and dry-run/apply classification. Replacement and bulk editing remain excluded.

## Verification Notes

- Cover generated/opaque/legacy refs, live/archive dependencies, all refusals, cycle matrices, ordering, YAML quoting, preview purity, rollback, and concurrent ledger changes.
- Run add then remove end to end and prove eligibility/state projection returns to the original semantic outcome.

## Implementation Notes
