---
id: T-194-add-explicit-archive-and-restore-commands
title: Add explicit archive and restore commands
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-and-restore-commands
dependencies:
    - T-193-move-task-storage-with-handle-bound-git
    - T-189-bind-archive-eligibility-to-verification
    - T-188-add-cancellation-provenance-and-dependency
    - T-192-protect-archived-history-across-all-semantic
updated_at: "2026-08-04T23:06:23Z"
---

# T-194-add-explicit-archive-and-restore-commands Add explicit archive and restore commands

## Description

Add one-task archive/restore over the move engine, preserving bytes, identity,
lifecycle, and aggregate state while enforcing eligibility/recovery.

## Acceptance

- Apply acquires non-delegated transaction ownership before
  resolver/eligibility/scanner reads and holds through move/validation/rollback;
  dry-run performs stable preflight with no writes.
- Delegated loop ownership refuses before move/stage with exact
  filesystem/index preservation.
- Moves preserve bytes/mode/frontmatter/status/IDs/timestamps/dependencies,
  including `loop_policy` and `loop_reason`, and never rewrite
  STATE/artifacts/specs/unrelated storage.
- Restore keeps terminal status and permits only narrow storage-derived-invalid
  recovery that fully validates live; edits require restore first.
- Destination/storage/eligibility/path/scan/Git/conflict/partial/rollback
  branches have exact classifications and one human/JSON outcome.

## Verification Notes

- Map criteria to pre-read/delegated races, every
  status/evidence/storage/dry-run/destination/path/Git/recovery branch, and
  unchanged refusal state, with explicit task-local loop-field comparisons.
- Manually archive/restore completed, cancelled, legacy-safe, adopted-debt, and
  inverse-uncommitted tasks comparing bytes/index/state.

## Implementation Notes
