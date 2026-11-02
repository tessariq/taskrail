---
id: T-447-respect-repository-manual-testing-policy-in
title: Respect repository manual-testing policy in autonomous skills
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies: []
updated_at: "2026-11-02T21:57:31Z"
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
