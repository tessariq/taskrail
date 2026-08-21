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

Make a freshly initialized local planning store usable by ordinary readers,
structural writers, and task authoring when it has no task files, without creating
committed state or weakening local ignore guarantees.

## Acceptance

- Explicit `init --local` durably creates the physical local `planning/tasks`
  directory even when the initial task corpus is empty.
- Empty-task readers and structural writers treat the initialized directory as a
  valid empty corpus rather than returning `not_initialized`.
- Initialization remains ignored and leaves ordinary Git status clean without
  creating a committed `planning/tasks` directory.
- Fault injection retains all-or-nothing local initialization behavior.

## Verification Notes

- Exercise fresh local init followed by status/path, spec add/activate, repair,
  and task authoring; run focused durability tests and the full repository gates.
