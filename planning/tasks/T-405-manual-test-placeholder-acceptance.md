---
id: T-405-manual-test-placeholder-acceptance
title: Define manual-test behavior when acceptance criteria are placeholders
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies: []
updated_at: "2026-09-11T16:28:47Z"
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
