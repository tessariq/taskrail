---
id: T-404-clarify-pre-start-cannot-proceed-branch
title: Clarify the cannot-proceed branch before start in autonomous skills
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies: []
updated_at: "2026-09-15T17:14:07Z"
completion_id: "c4bd14664c65070993d7c2141b4bfd6c"
last_verification_id: "739effec0afc14b2937d7ad10e39f930"
last_verification_result: pass
last_verified_at: "2026-09-15T17:14:07Z"
last_verified_completion_id: "c4bd14664c65070993d7c2141b4bfd6c"
---

# T-404-clarify-pre-start-cannot-proceed-branch Clarify the cannot-proceed branch before start in autonomous skills

## Description

`autonomous-backlog` and `autonomous-task` both say to apply the sizing rubric
before `start` and "Stop for reviewed decomposition or clarification", and later
"If work cannot proceed, run block ... then verify --result fail". On the same
placeholder task the v0.5.0 candidates split: `autonomous-backlog-committed`
judged it unsizable and ran `start` anyway because the request asked for a
positive workflow; `autonomous-task-committed` and `autonomous-task-local` ran
`block` plus a failing verify without starting; `autonomous-backlog-local`
stopped read-only.

Outcome: one unambiguous instruction for a todo task that fails the sizing
gate, which a request's phrasing cannot override.

## Acceptance

- Both skills state the same action for a gate-failing todo task (read-only
  stop, or block with a reason) and that the gate precedes any `start`
  regardless of request wording.
- `task check:skills` passes after regeneration.

## Verification Notes

- Skill contract test plus the matching eval case expectation.

## Implementation Notes

- 2026-09-15T17:14:02Z: Both autonomous skills now state one unambiguous pre-start sizing-gate action: never start a gate-failing todo task; block with a reason, record a failing verify, and stop, regardless of request wording. Contract test TestFullTaskSkillsShareOnePreStartGateBranch pins identical gate-branch text in both skills; four autonomous skill-eval case expectations updated to match; committed skill copies regenerated with parity verified.
- 2026-09-15T17:14:07Z: verification pass id 739effec0afc14b2937d7ad10e39f930 previous none completion c4bd14664c65070993d7c2141b4bfd6c
