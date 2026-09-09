---
id: T-394-make-skill-eval-baselines-executable
title: Make skill-evaluation baseline arms executable on v0.4.0
status: completed
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-09T18:52:55Z"
completion_id: "8c1d171344301d041da083315ae10129"
last_verification_id: "dc98fa5da0f3805706705c2956348135"
last_verification_result: pass
last_verified_at: "2026-09-09T18:52:55Z"
last_verified_completion_id: "8c1d171344301d041da083315ae10129"
---

# T-394-make-skill-eval-baselines-executable Make skill-evaluation baseline arms executable on v0.4.0

## Description

TODO: describe the outcome's invariant and relevant spec section.

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes

- 2026-09-09T18:52:55Z: Baseline classification now requires the skill and the case storage mode to exist in v0.4.0, and a baseline-required scenario is checked against a maintainer-owned v0.4.0 command surface. The release answers an unknown subcommand with its parent help and exit zero, so an unchecked probe graded pass while observing nothing; the committed task-readability probe now reads the committed file through git, which both arms run.
- 2026-09-09T18:52:55Z: verification pass id dc98fa5da0f3805706705c2956348135 previous none completion 8c1d171344301d041da083315ae10129
