---
id: T-442-report-task-not-found-before-active-conflict
title: Report task_not_found before an active-task conflict in start
status: todo
priority: low
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies: []
updated_at: "2026-10-31T11:39:15Z"
---

# T-442-report-task-not-found-before-active-conflict Report task_not_found before an active-task conflict in start

## Description

Bug. While another task is in progress, `taskrail start T-999-missing` returns
`invalid_status` "task ... is already active" instead of `task_not_found`. The
caller is told to finish another task when the real error is a wrong ID. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- `start <unknown-id>` returns `task_not_found` whether or not another task is active.

## Verification Notes

- Smoke test with one active task and an unknown ID.

## Implementation Notes
