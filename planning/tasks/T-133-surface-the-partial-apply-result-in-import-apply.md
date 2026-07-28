---
id: T-133-surface-the-partial-apply-result-in-import-apply
title: Surface the partial apply result in import --apply CLI output
status: todo
priority: medium
spec_ref: specs/v0.2.0.md#taskrail-import
dependencies:
    - T-132-report-partially-created-tasks-when-import-apply
updated_at: "2026-07-28T12:42:40Z"
---

# T-133-surface-the-partial-apply-result-in-import-apply Surface the partial apply result in import --apply CLI output

## Description

Discovered reviewing T-132. The service layer now populates `ApplyDraftResult`
on a partial apply, but the CLI discards it: `cmd/taskrail/import.go` returns the
error from `ApplyImportDraft` without calling `printApplyResult`, so `--json`
emits nothing at all on a mid-write failure and text mode shows only the prose
wrapper — never the per-task `path` values a "review before retrying" workflow
needs to parse.

Decide and implement how a partial result reaches the operator: emitting the
result before returning the error keeps `--json` parseable, but a success-shaped
envelope on a failed run is a contract change that needs an explicit shape (for
example a `partial: true` marker) so scripts cannot mistake it for a clean apply.

Follow-up derived from T-132-report-partially-created-tasks-when-import-apply's verification or discovery.

## Acceptance

- `taskrail import --apply --json` emits a parseable envelope naming the spec path
  and the tasks written before a mid-write failure, distinguishable from a
  successful apply.
- Text mode reports the same written artifacts alongside the existing partial-apply
  error wrapper.
- Exit status stays non-zero on a partial apply; successful applies are unchanged.
- Automated coverage: a CLI-level test forcing a mid-write failure asserts both the
  emitted output and the non-zero exit.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
