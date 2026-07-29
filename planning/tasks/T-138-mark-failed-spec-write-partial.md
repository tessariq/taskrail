---
id: T-138-mark-failed-spec-write-partial
title: Mark a failed imported-spec write as a partial apply
status: todo
priority: low
spec_ref: specs/v0.2.0.md#taskrail-import
dependencies:
    - T-133-surface-the-partial-apply-result-in-import-apply
updated_at: "2026-07-29T10:24:44Z"
---

# T-138-mark-failed-spec-write-partial Mark a failed imported-spec write as a partial apply

## Description

Discovered reviewing T-133. The partial-apply reporting path only covers a failure
inside `createDraftTasks`: `ApplyImportDraft` returns a zero-value
`ApplyDraftResult{}` when `writeImportedSpec` fails
(`internal/taskrail/import_apply.go`), so `Partial` stays false and the CLI emits
nothing — even though `os.WriteFile` may already have created (or truncated) the
spec file at `specs/<source>.md`, and `ensureDir` may have created its parent.

Decide whether that residual case deserves the same treatment: report the path the
apply touched so the operator knows the tree moved, or establish that a failed
spec write leaves nothing worth reporting (an empty file is still a file a retry
would find, and `isImportedSpec` would not recognize it as a re-appliable orphan).

Follow-up derived from T-133-surface-the-partial-apply-result-in-import-apply's verification or discovery.

## Acceptance

- A failed imported-spec write either reports the touched path through the same
  partial-apply channel (`partial: true` envelope plus the error wrapper) or is
  documented as leaving nothing to review, with the reason.
- If an empty/truncated spec file can survive the failure, a retry of the same
  draft is either accepted (recognized as an orphan import) or fails with an
  actionable message naming the file.
- Automated coverage for whichever behavior is chosen.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
