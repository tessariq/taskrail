---
id: T-143-preflight-task-rename-validity-before-applying
title: Preflight task rename validity before applying writes
status: todo
priority: high
spec_ref: specs/v0.4.0.md#task-rename-and-re-slug
dependencies:
    - T-129-preview-post-apply-validity-in-task-rename-dry-run
updated_at: "2026-07-29T13:04:09Z"
---

# T-143-preflight-task-rename-validity-before-applying Preflight task rename validity before applying writes

## Description

`task rename` validates only after applying its coupled writes. Validation returns a
structured invalid result without an error, so a repository violation the rename
does not heal can leave the rename applied with `validation.valid=false`. The spec
requires no partial change when validation would fail; preview the exact post-rename
state before writes and refuse invalid outcomes.

## Acceptance

- Apply mode validates the same in-memory post-rename state as dry run before any
  filesystem mutation.
- If that preview is invalid, the command exits non-zero, reports the violations,
  and leaves the id, filename, body heading, inbound dependencies, and `STATE.md`
  byte-identical.
- A rename that heals the only existing violation remains allowed and applies
  atomically.
- Existing returned-I/O-error rollback and collision behavior stay covered.

## Verification Notes

- T-140 sandbox evidence renamed `T-001-visible-title` to `T-001-renamed` and
  returned `applied=true` with an unrelated invalid `spec_ref` still present.

## Implementation Notes

- Reuse `renamePreview`; do not add a second model of the coupled change set.
