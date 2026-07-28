---
id: T-132-report-partially-created-tasks-when-import-apply
title: Report partially created tasks when import apply fails mid-write
status: todo
priority: medium
spec_ref: specs/v0.2.0.md#taskrail-import
dependencies:
    - T-128-task-new-title-portability
updated_at: "2026-07-28T12:28:52Z"
---

# T-132-report-partially-created-tasks-when-import-apply Report partially created tasks when import apply fails mid-write

## Description

Discovered reviewing T-128. `createDraftTasks` returns `nil, fmt.Errorf(...)` when
one draft task fails mid-loop, discarding the `created` slice for the tasks it had
already written. `ApplyImportDraft` then assigns that nil to `result.Tasks`, so
`describeWrittenArtifacts` finds nothing and the documented "partial apply already
wrote X — review before retrying" wrapper never fires — even though task files are
on disk and `STATE.md` counts moved.

T-128 removed the guard-mismatch trigger by mirroring the title-portability check
in `preflightImportDraft`, so pre-flight again covers every check `CreateTask`
applies. The residual trigger is the one the comment on `ApplyImportDraft` names:
a mid-write I/O failure. That path still reports silently.

Return the partial `created` slice alongside the error so the existing
partial-apply wrapper can do its job.

Follow-up derived from T-128-task-new-title-portability's verification or discovery.

## Acceptance

- A mid-write failure in `createDraftTasks` still returns the tasks created before
  the failure, so `ApplyImportDraft` reports them in `result.Tasks`.
- The resulting error carries the `partial apply already wrote ...` wrapper naming
  those task ids (and the spec path, when one was written).
- Successful applies are unchanged.
- Automated coverage: a test forcing a mid-loop failure asserts both the partial
  result and the wrapped message.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
