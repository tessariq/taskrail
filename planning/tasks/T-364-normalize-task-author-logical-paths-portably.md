---
id: T-364-normalize-task-author-logical-paths-portably
title: Normalize task author logical paths portably
status: completed
priority: high
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-161-apply-reviewed-task-bodies-with-compare-and-swap
updated_at: "2026-08-25T20:35:54Z"
completion_id: "7ff75a20dd0ab94ffc34358271d6cbe6"
last_verification_id: "4cca21f9c62350ab582d5eae9a84c154"
last_verification_result: pass
last_verified_at: "2026-08-25T20:35:54Z"
last_verified_completion_id: "7ff75a20dd0ab94ffc34358271d6cbe6"
---

# T-364-normalize-task-author-logical-paths-portably Normalize task author logical paths portably

## Description

Make task-author proposal validation interpret repository-relative input as a
slash-delimited logical path on every supported operating system. This restores
the documented invalid-proposal response for ignored artifact paths on native
Windows without weakening canonical-path or repository-boundary checks.

## Acceptance

- A1. A canonical slash-delimited proposal path is accepted for validation on
  native Windows and other supported platforms.
- A2. A proposal below the logical planning artifacts directory is rejected as
  `invalid_proposal`, while non-canonical and platform-native separator input
  remains `invalid_arguments`.

## Verification Notes

- A1-A2: Run the focused task-author tests locally and the complete cross-platform
  GitHub Actions matrix; the native-Windows artifact-body case must report
  `invalid_proposal`.

## Implementation Notes

- 2026-08-25T20:35:49Z: Use platform-independent logical path normalization for task-author proposals.
- 2026-08-25T20:35:54Z: verification pass id 4cca21f9c62350ab582d5eae9a84c154 previous none completion 7ff75a20dd0ab94ffc34358271d6cbe6
