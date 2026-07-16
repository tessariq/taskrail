---
id: T-108-next-filter-idle-selection-to-active-spec-tasks
title: 'next: filter idle selection to active-spec tasks'
status: todo
priority: high
spec_ref: specs/v0.4.0.md#active-spec-filtered-next-selection
dependencies:
    - T-103
updated_at: "2026-07-16T09:50:17Z"
---

# T-108-next-filter-idle-selection-to-active-spec-tasks next: filter idle selection to active-spec tasks

## Description

`taskrail next` currently ranks every eligible `todo` task and only warns when the
selected task points outside `STATE.md`'s active spec. Change idle selection so it
filters candidates to `active_spec_path` before priority/id ranking, per
`specs/v0.4.0.md#active-spec-filtered-next-selection`. Keep the existing warning
when an already-active `in_progress` task points outside the active spec.

## Acceptance

- With no active task, `next` considers only `todo` tasks whose dependencies are
  resolved and whose `spec_ref` path matches `STATE.md`'s `active_spec_path`.
- A higher-priority or lower-id eligible task from an older/other spec is skipped
  when an active-spec eligible task exists; selection still ranks active-spec
  candidates by priority, then stable task id.
- If only older/other-spec tasks are eligible, human `next` reports no eligible
  task and `next --json` exposes structured detail that lets agents see old-spec
  runnable work was skipped rather than parsing text.
- If `next` returns an already-active `in_progress` task whose `spec_ref` points
  outside the active spec, it still returns that task and includes the existing
  `selected_non_active_spec` warning in human and JSON output.
- `status` reuses the same filtered idle selection without writing `planning/STATE.md`
  or task files; its active-task warning behavior remains aligned with `next`.
- README, workflow docs, and the Unreleased changelog describe `next` as
  active-spec-filtered rather than advisory-warning-only.
- Automated coverage includes service-level selector tests and CLI smoke tests for
  active-spec selection, skipped old-spec candidates, no active-spec eligible work,
  and the active old-spec continuation warning.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- T-103 shipped the intermediate advisory warning behavior; this task revises idle
  selection semantics while preserving the warning for already-active drift.
