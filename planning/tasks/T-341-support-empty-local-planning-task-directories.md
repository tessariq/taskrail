---
id: T-341-support-empty-local-planning-task-directories
title: Support empty local planning task directories
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-288-inspect-local-planning-storage-read-only
updated_at: "2026-08-21T09:50:50Z"
---

# T-341-support-empty-local-planning-task-directories Support empty local planning task directories

## Description

Make a freshly initialized local planning store usable by ordinary readers and task authoring when it has no task files, without creating committed state or weakening local ignore guarantees.

## Acceptance

- The follow-up issue described by verification is resolved.
- Verification evidence is updated.

## Verification Notes

- Re-run task-scoped verification after implementing the fix.
