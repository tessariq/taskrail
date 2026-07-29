---
id: T-141-slug-titled-tasks-created-by-import-and-verify
title: Slug titled tasks created by import and verify follow-ups
status: todo
priority: high
spec_ref: specs/v0.4.0.md#slugged-task-creation
dependencies:
    - T-095
updated_at: "2026-07-29T13:03:42Z"
---

# T-141-slug-titled-tasks-created-by-import-and-verify Slug titled tasks created by import and verify follow-ups

## Description

The v0.4.0 goal says CLI-authored tasks use the slugged house style, but only
`task new` supplies the title as a slug source. `import --apply` and
verify-created follow-ups have titles yet still create bare `T-<n>` ids and
filenames. Route those creation paths through the same deterministic title-derived
slug behavior without changing their dependency or provenance semantics.

## Acceptance

- A titled task promoted by `import --apply` receives a capped, title-derived
  slugged id and matching filename.
- A titled follow-up created by `verify --create-followup` receives the same
  slugged form.
- Dependency translation, follow-up parent dependencies, JSON paths/ids, warnings,
  portability checks, and post-write validation remain correct.
- Tests cover both paths, including a title that normalizes to an empty slug.

## Verification Notes

- Reproduced by the T-140 release-gate sandbox: importing `Imported readable task`
  created bare `T-003.md`.

## Implementation Notes

- Keep one shared title-to-id policy rather than duplicating slug logic in import
  and verification.
