---
id: T-154-pin-all-workflow-actions-to-commit-shas
title: Pin every workflow action to an immutable commit SHA
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#goals
dependencies:
    - T-153-dependabot-compatible-mise-action-pin-guard
updated_at: "2026-08-01T08:24:20Z"
---

# T-154-pin-all-workflow-actions-to-commit-shas Pin every workflow action to an immutable commit SHA

## Description

Only `jdx/mise-action` is pinned to a commit SHA. `actions/checkout@v7`,
`actions/setup-go@v7`, and `goreleaser/goreleaser-action@v7` float on mutable
major tags, so whoever controls those tags can change what CI and the release
pipeline execute without any diff in this repository — the exact exposure the
mise-action pin was introduced to close. T-153 made the pin guard property-based,
so the same properties now generalize to every action at no maintenance cost.

Pin all three at the commit their `@v7` tag currently resolves to, so the change
is behavior-neutral, and extend the guard to every file under
`.github/workflows/` so a new workflow or a reverted pin fails the build.

## Acceptance

- Every `uses:` step in every `.github/workflows/*.yml` pins an immutable
  40-character commit SHA annotated with `# vX.Y.Z`.
- The guard enumerates the workflow directory rather than a hardcoded file list,
  so a newly added workflow is covered without editing the test.
- A floating tag or an unannotated pin in any workflow fails the guard.
- The mise-action-specific rules (version floor, one revision across workflows)
  still apply on top of the general contract.
- The pinned SHAs equal what each `@v7` tag resolves to today, so no action
  behavior changes with this commit.

## Verification Notes

- Tag-to-commit resolution recorded in the implementation notes below.

## Implementation Notes

- 2026-08-01T08:24:16Z: verification pass
