---
id: T-405-manual-test-placeholder-acceptance
title: Define manual-test behavior when acceptance criteria are placeholders
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies: []
updated_at: "2026-09-15T20:17:22Z"
completion_id: "14db8bd3f8d890c2b904710fc5bd3346"
last_verification_id: "160d9f06034af761bfa00ecf7c19fe18"
last_verification_result: pass
last_verified_at: "2026-09-15T20:17:22Z"
last_verified_completion_id: "14db8bd3f8d890c2b904710fc5bd3346"
---

# T-405-manual-test-placeholder-acceptance Define manual-test behavior when acceptance criteria are placeholders

## Description

`autonomous-manual-test` derives test steps from acceptance criteria but says
nothing when they are missing or placeholder text. In both v0.5.0 candidate arms
(`autonomous-manual-test-committed`, `-local`) the agent used the evaluation
request's expected observation as criteria and reported verdict pass, disclosed;
the v0.4.0 baseline reported fail.

Outcome: the skill defines the outcome when acceptance criteria are absent.

## Acceptance

- The skill states that missing or placeholder acceptance criteria yield a
  stated non-pass outcome (fail or stop) and that improvised criteria never
  support a pass.
- `task check:skills` passes after regeneration.

## Verification Notes

- Skill contract test; eval case expectation update.

## Implementation Notes

- 2026-09-15T20:17:17Z: Skill states missing/placeholder acceptance criteria yield a stated non-pass outcome (fail or stop) with stop-writing-no-report semantics; improvised criteria never support a pass. Contract test added red-green; both eval case expectations updated; committed mirrors regenerated; manual sandbox evidence recorded.
- 2026-09-15T20:17:22Z: verification pass id 160d9f06034af761bfa00ecf7c19fe18 previous none completion 14db8bd3f8d890c2b904710fc5bd3346
