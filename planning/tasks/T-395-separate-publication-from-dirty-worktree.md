---
id: T-395-separate-publication-from-dirty-worktree
title: Stop grading durable review publication as a dirty worktree
status: completed
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-09T18:53:19Z"
completion_id: "f28d85689931c644f542fbea961ce085"
last_verification_id: "33c2e5761dbb10ac621077fc2e8d5093"
last_verification_result: pass
last_verified_at: "2026-09-09T18:53:19Z"
last_verified_completion_id: "f28d85689931c644f542fbea961ce085"
---

# T-395-separate-publication-from-dirty-worktree Stop grading durable review publication as a dirty worktree

## Description

TODO: describe the outcome's invariant and relevant spec section.

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes

- 2026-09-09T18:53:06Z: verification pass id de6fe3c0ce0c92786fc3e5c81f21ae2c previous none completion none
- 2026-09-09T18:53:19Z: Skills whose contract is to publish a durable review bundle now grade under git-publication-only: clean tracked state before and after, an unmoved HEAD, and an unchanged ref listing, while new untracked paths are admitted. Under the previous blanket clean-worktree assertion those skills failed for producing their own required output, and passed only when a run happened not to reach the publish step.
- 2026-09-09T18:53:19Z: verification pass id 33c2e5761dbb10ac621077fc2e8d5093 previous none completion f28d85689931c644f542fbea961ce085
