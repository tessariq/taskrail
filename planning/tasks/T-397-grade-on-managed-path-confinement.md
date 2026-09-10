---
id: T-397-grade-on-managed-path-confinement
title: Grade skill evaluations on managed-path confinement
status: completed
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-10T09:57:30Z"
completion_id: "c0aef0e5392430db1e46e5a8893ef8dc"
last_verification_id: "777735f8d35990c4c974920029987356"
last_verification_result: pass
last_verified_at: "2026-09-10T09:57:30Z"
last_verified_completion_id: "c0aef0e5392430db1e46e5a8893ef8dc"
---

# T-397-grade-on-managed-path-confinement Grade skill evaluations on managed-path confinement

## Description

TODO: describe the outcome's invariant and relevant spec section.

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes

- 2026-09-10T09:57:17Z: Replaced the blanket clean-worktree assertion with git-managed-paths-only over an observed changed-path listing, kept git-publication-only as a narrowed tracked-bytes assertion for the review skills, and taught taskrail-validation-pass to read both the v0.5 and legacy v0.4.0 validation envelopes.
- 2026-09-10T09:57:30Z: verification pass id 777735f8d35990c4c974920029987356 previous none completion c0aef0e5392430db1e46e5a8893ef8dc
