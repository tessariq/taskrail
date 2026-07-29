---
id: T-149-harden-release-version-and-changelog-notes-guards
title: Harden release version and changelog notes guards
status: completed
priority: high
spec_ref: specs/v0.4.0.md#goals
dependencies:
    - T-015
updated_at: "2026-07-29T17:28:21Z"
---

# T-149-harden-release-version-and-changelog-notes-guards Harden release version and changelog notes guards

## Description

The direct release build reports `v0.4.0`, but GoReleaser injects its normalized
version without the `v` prefix. Separately, an empty changelog release section
falls back to the tag text, so the workflow's non-empty-file guard accepts release
notes with no actual changelog content. Fix both release-blocking guard gaps.

## Acceptance

- GoReleaser-built snapshot and tag-version binaries report `v<version>`, matching
  `task release`, install docs, and prior release contracts.
- A repository test or toolchain guard asserts the GoReleaser ldflag retains the
  `v` prefix.
- Changelog extraction/guarding fails for a missing or empty release section; it
  never substitutes the tag as apparent release notes on the publish path.
- Existing dated, non-empty sections extract exactly their body, and release
  workflow tests cover missing, empty, and populated cases.
- `goreleaser check`, a clean snapshot, direct release build, and changelog guards
  all pass before T-140 is rerun.

## Verification Notes

- T-140 snapshot binary reported `0.3.1-snapshot`; an empty-section sandbox passed
  the heading guard and produced fallback notes `v0.4.0`.

## Implementation Notes

- Also correct the stale platform comment in `.goreleaser.yaml` and make the
  Taskfile release-version example version-neutral while touching this surface.
- 2026-07-29T17:28:02Z: verification pass
