---
id: T-457-map-layout2-observe-slash-path
title: Observe migrated transactions with a slash path in tests
status: in_progress
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-456-map-windows-move-collision
updated_at: "2026-11-25T23:15:00Z"
---

# T-457-map-layout2-observe-slash-path Observe migrated transactions with a slash path in tests

## Description

Follow-up from T-455-exercise-windows-durable-paths. Native Windows CI run
45711904788 at `474eab0` ran the layout-2 migration tests for the first time.
Those tests were gated on the old directory-sync probe until T-455. Ten
subtests of `TestApplyNoteDecisions` and `TestLayout2UpgradeGitignoreMatrix`
failed with `transactions retained after the migration: [] err = invalid
relative filesystem path: ".git\\taskrail\\transactions"`.

The migration applied. The failure is in the shared assertion
`assertMigratedToLayout2`, which passes a `filepath.Join` path to
`durablefs.ObserveTree`. That API takes slash-separated relative paths, as every
production caller does, so the backslash-joined path is rejected on Windows.

Outcome: the assertion observes the transactions directory with a slash path,
so it checks the same contract on every platform.

## Acceptance

- `assertMigratedToLayout2` passes a slash-separated relative path to
  `durablefs.ObserveTree`, and its retained-transaction assertion is unchanged.
- The layout-2 migration tests pass on Linux, and run and pass on native
  Windows NTFS.

## Verification Notes

- Regression evidence: the ten failing subtests in run 45711904788.
- `gofmt -l .`, `go vet ./...` (linux, windows, darwin), Windows test build,
  targeted Linux tests.
- Native confirmation needs the next approved Windows run.

## Implementation Notes
