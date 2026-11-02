---
id: T-447-respect-repository-manual-testing-policy-in
title: Respect repository manual-testing policy in autonomous skills
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies: []
updated_at: "2026-11-02T23:02:27Z"
completion_id: "900a7bcb6dfc23867bae3ef78760caf5"
last_verification_id: "1ecf33c7d1cb0056811714432d518dc6"
last_verification_result: pass
last_verified_at: "2026-11-02T23:02:27Z"
last_verified_completion_id: "900a7bcb6dfc23867bae3ef78760caf5"
---

# T-447-respect-repository-manual-testing-policy-in Respect repository manual-testing policy in autonomous skills

## Description

Adopter skill audit finding P5 reports that autonomous-task and
autonomous-backlog condition manual testing on visible behavior even when the
adopting repository requires it for every task. Both packaged skills currently
say "sandbox-first manual testing for visible workflow behavior."

Make the full-task flows respect the repository's documented manual-testing
policy. Where no policy exists, retain the visible-behavior fallback. This is
guidance precedence, not a new configuration surface or a change to the CLI.

## Acceptance

- Both skills explicitly follow the repository's manual-testing policy,
  including an unconditional requirement for otherwise non-visible changes.
- With no repository policy, both retain manual testing for visible workflow
  behavior; do not make every adopter run manual tests on every task.
- Preserve lifecycle order, evidence requirements, and sandbox safety limits.
- Regenerate committed skill copies from the package and pass parity checks.

## Verification Notes

- Exercise guidance against two adopter scenarios: a non-visible change with
  mandatory manual testing, and a visible change without a repository policy.
  Expect manual testing in both; also check a non-visible change without policy
  does not gain an unconditional requirement. Keep scenario evidence under the
  reported transient artifact root.
- Add or extend focused skill-contract coverage for policy precedence and the
  fallback; run it and `task check:skills` after regeneration.

## Implementation Notes

- 2026-11-02T23:02:27Z: Both full-task skills now follow repository manual-testing policy, including non-visible changes; policy-free fallback remains visible workflow behavior. Regenerated mirrors, red/green contract coverage, three manual guidance scenarios, simplification and independent review passed. Full Linux tests and vet passed; known Windows baseline remains outside this task.
- 2026-11-02T23:02:27Z: verification pass id 1ecf33c7d1cb0056811714432d518dc6 previous none completion 900a7bcb6dfc23867bae3ef78760caf5
