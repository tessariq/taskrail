---
id: T-153-dependabot-compatible-mise-action-pin-guard
title: Make the mise-action pin guard dependabot-compatible
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#goals
dependencies: []
updated_at: "2026-08-01T08:13:03Z"
---

# T-153-dependabot-compatible-mise-action-pin-guard Make the mise-action pin guard dependabot-compatible

## Description

`TestWorkflowsPinMiseAction` in `internal/toolchain/ci_test.go` asserts the
mise-action reference equals one hardcoded commit SHA. That literal is a second
source of truth for a pin Dependabot can only move in workflow YAML, so every
routine `jdx/mise-action` bump fails the `build-test` matrix by construction
(observed on PR #4, v4.2.1 -> v4.2.3). The guard must assert the properties the
pin buys instead of one revision, so an automated bump stays green while a
floating tag, an unannotated pin, or a partial hand-edit still fails.

## Acceptance

- Every workflow `jdx/mise-action` reference must be pinned to an immutable
  40-character commit SHA; a floating tag (`@v4`, `@main`) fails the guard.
- Every pin must carry a `# vX.Y.Z` version annotation, and the annotated version
  must not drop below the reviewed v4.2.1 floor that stopped `[env]` PATH export.
- All workflows must pin the same reference; a divergent SHA in one workflow
  fails the guard and names the offending files.
- A Dependabot-style bump that rewrites every `uses:` line to a new SHA and
  version comment passes without editing Go source.
- No commit SHA is hardcoded in `internal/toolchain`.

## Verification Notes

- Root cause traced from the PR #4 `build-test` failures
  (`ci_test.go:72`, all four matrix runners).

## Implementation Notes

- 2026-08-01T08:12:55Z: verification pass
