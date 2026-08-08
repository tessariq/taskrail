---
id: T-197-extend-loop-postflight-for-storage-and
title: Extend loop postflight for storage and cancellation
status: todo
priority: high
spec_ref: specs/v0.6.0.md#unified-workflow-and-loop-integration
dependencies:
    - T-194-add-explicit-archive-and-restore-commands
    - T-188-add-cancellation-provenance-and-dependency
    - T-196-integrate-stable-references-with-rename-prompts
    - T-314-integrate-loop-continuation-and-terminal
updated_at: "2026-08-04T23:06:23Z"
---

# T-197-extend-loop-postflight-for-storage-and Extend loop postflight for storage and cancellation

## Description

Extend the v0.5 loop's frozen ledger and exhaustive outcome model for storage
mutations and the exact cancelled_handoff operator boundary.

## Acceptance

- Loop captures stable ref, full ID, storage, `loop_policy`, and `loop_reason`
  for every task and rejects any pre-existing loop-field change, unrelated task
  change, or live/archive move. The selected task may change only through the
  outcome-specific v0.5 write set or the cancellation write set below. Any allowed
  new live follow-up omits loop fields for implicit hold; a new archived follow-up
  is invalid.
- Built-in prompt never invokes cancel/archive/restore and archive/restore
  commands refuse delegated ownership independently.
- Cancelled_handoff requires canonical newly added provenance, no open
  dependent, valid task/state, unchanged task-local loop fields, same attached
  ref, descendant HEAD/local commit containing only allowed cancellation/state
  reconciliation, clean tree, no new task, and no process.
- Verification is not cancellation evidence; outcome always exits non-zero,
  emits prior status/reason/time/ref/ID/Git/next-action diagnostics, and never
  selects again.
- Dirty/uncommitted/malformed/stranded/extra/archived/child-failed cancellation
  is invalid_postflight and still reports attempted provenance.

## Verification Notes

- Map every predicate to
  storage/cancellation/ref/ancestry/commit/process/exit/diagnostic fixtures,
  including loop-field mutation, implicit-hold follow-up, create-then-cancel,
  and child-failed cases.
- Persist manual archive-mutation and valid/invalid cancelled-handoff reports.

## Implementation Notes
