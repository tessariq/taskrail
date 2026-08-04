---
id: T-187-create-opaque-tasks-through-commands-and
title: Create opaque tasks through commands and ImportDraft v3
status: todo
priority: high
spec_ref: specs/v0.6.0.md#portable-opaque-task-ids
dependencies:
    - T-186-resolve-layout-3-identity-collisions-with-a
    - T-179-resolve-stable-task-references-across-every
    - T-177-validate-opaque-ids-and-importdraft-v3-structure
    - T-163-validate-and-apply-importdraft-v2-transactionally
updated_at: "2026-08-04T23:06:23Z"
---

# T-187-create-opaque-tasks-through-commands-and Create opaque tasks through commands and ImportDraft v3

## Description

Wire validated immutable opaque IDs into layout-3 task creation and reviewed
transactional ImportDraft v3 without changing generated defaults.

## Acceptance

- `task new --id` requires layout 3 and enforces validation, title independence,
  slug conflict, archive/sibling collisions, and immutable/rename refusal.
- V3 import inherits v2 review/digest/body/all-or-none semantics and performs
  whole-draft opaque/generated/dependency/archive/allocation preflight.
- V3 generated demand uses the shared complete-ledger allocator API; exhaustive
  inherited-entry-point adoption remains writer integration.
- Explicit opaque command/all-opaque v3 import succeed at generated exhaustion;
  mixed generated demand fails before every write.
- Opaque writes remain live, byte-exact, allocation-neutral, and unavailable on
  source layouts.

## Verification Notes

- Map criteria to command/import/rename/layout, archive/index collisions,
  exhaustion, concurrency, and mixed drafts.
- Manually create/resolve OPS, JIRA, and T-jira IDs on native Windows and smoke
  macOS/Linux.

## Implementation Notes
