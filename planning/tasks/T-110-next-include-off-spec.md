---
id: T-110-next-include-off-spec
title: Add next --include-off-spec recovery flag
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#active-spec-filtered-next-selection
dependencies:
    - T-108-next-filter-idle-selection-to-active-spec-tasks
updated_at: "2026-07-17T11:20:55Z"
---

# T-110-next-include-off-spec Add next --include-off-spec recovery flag

## Description

T-108 filtered `next` idle selection to the active spec. Older-spec open tasks are
now recoverable only via explicit `start <id>`, which requires the operator to
already know the id. `status` (T-106) lists away-from-active work for humans, but
an agent loop with an empty active-spec backlog has no single command that returns
a runnable off-spec task.

Add `taskrail next --include-off-spec` (per the v0.4.0 Active-Spec Filtered Next
Selection amendment): a one-shot opt-out of the active-spec filter that ranks
eligible `todo` tasks across all specs with the original ranking and flags any
off-active-spec pick.

## Acceptance

- `next --include-off-spec` considers eligible `todo` tasks regardless of
  `spec_ref`, ranked by the existing order (status, resolved dependencies,
  priority, stable task id). Default `next` (no flag) is unchanged — still
  active-spec-filtered.
- When the selected task points away from `active_spec_path`, human output flags
  it (e.g. an off-spec marker plus the task's `spec_ref`), and `--json` exposes a
  structured flag distinguishing an active-spec pick from an off-spec pick.
- The flag never bypasses `start`'s transition rules and writes no more state than
  a normal `next` selection probe (the existing `next_action`/`updated_at`
  update); an already-active `in_progress` task still wins the slot with its
  warning.
- Tests: with only older-spec `todo` tasks eligible, plain `next` reports none
  while `next --include-off-spec` returns the ranked off-spec task with the flag
  set (human and `--json`).

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
