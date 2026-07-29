---
id: T-136-layout-marker-error-path-form
title: Make layout marker error paths consistently repo-relative
status: todo
priority: low
spec_ref: specs/v0.4.0.md#layout-compatibility-beyond-init
dependencies:
    - T-131-layout-marker-read-dedup
updated_at: "2026-07-29T09:23:32Z"
---

# T-136-layout-marker-error-path-form Make layout marker error paths consistently repo-relative

## Description

Follow-up discovered while implementing T-131-layout-marker-read-dedup.

`readLayoutFile` in `internal/taskrail/paths.go` reports the same file two ways
depending on which branch fails: the read error uses `relPath(root, path)`
(`.taskrail/config.yml`) while the parse error uses the absolute `path`
(`/abs/repo/.taskrail/config.yml`). T-131 preserved the asymmetry deliberately —
it predates the fold, was identical in both original readers, and normalizing it
would have been a behavior change outside a pure-hygiene dedup.

Everywhere else Taskrail reports repo-relative paths so output stays portable and
does not leak the producer's absolute filesystem layout. The parse branch looks
like an oversight rather than a decision, but it is adopter-facing error text, so
confirm before changing: an adopter may be matching on it.

## Acceptance

- Decide whether the absolute path in the parse-error branch is intended.
- If it is not: `readLayoutFile` reports the marker path in one form for both the
  read and the parse branch, matching the repo-relative form used elsewhere, with
  test coverage pinning the parse-error path form.
- If it is intended: close as cancelled with the reason recorded, and leave a
  comment at the branch so the asymmetry is not re-flagged.
- No other error wording changes; the `layout config` versus `layout marker` label
  distinction stays exactly as T-131 left it.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
