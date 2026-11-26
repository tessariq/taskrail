---
id: T-457-map-layout2-observe-slash-path
title: Observe migrated transactions with a slash path in tests
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-456-map-windows-move-collision
updated_at: "2026-11-26T00:57:41Z"
completion_id: "0a9a2d56811753bcfbf3820b822d66bd"
last_verification_id: "8f5f6057ca563648b225450cb063f146"
last_verification_result: pass
last_verified_at: "2026-11-26T00:57:41Z"
last_verified_completion_id: "0a9a2d56811753bcfbf3820b822d66bd"
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

- 2026-11-25T23:17:31Z: Fix committed; awaiting native Windows confirmation of the layout-2 migration tests in the next approved run
- 2026-11-26T00:57:38Z: Native Windows run 45722268952 at c674115 finished; confirmation evidence available.
- 2026-11-26T00:57:40Z: Native Windows run 45722268952 at c674115: internal/taskrail ok (non-verbose; per-test result inferred from package success).
- 2026-11-26T00:57:41Z: verification pass id 8f5f6057ca563648b225450cb063f146 previous none completion 0a9a2d56811753bcfbf3820b822d66bd
