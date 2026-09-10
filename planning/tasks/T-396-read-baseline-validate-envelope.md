---
id: T-396-read-baseline-validate-envelope
title: Read the v0.4.0 validate envelope in baseline arms
status: completed
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-10T09:28:48Z"
completion_id: "51db02d3848002b9c811c7407d4b170c"
last_verification_id: "9c6047f0b8ac639f7861905ca73ca3ae"
last_verification_result: pass
last_verified_at: "2026-09-10T09:28:48Z"
last_verified_completion_id: "51db02d3848002b9c811c7407d4b170c"
---

# T-396-read-baseline-validate-envelope Read the v0.4.0 validate envelope in baseline arms

## Description

TODO: describe the outcome's invariant and relevant spec section.

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes

- 2026-09-10T09:28:48Z: The observed validation fact now reads the envelope the arm under test emits: the pinned v0.4.0 baseline predates the common JSON envelope and answers with a bare result, while the candidate must answer in its own envelope so a regression there cannot pass through a legacy fallback.
- 2026-09-10T09:28:48Z: verification pass id 9c6047f0b8ac639f7861905ca73ca3ae previous none completion 51db02d3848002b9c811c7407d4b170c
