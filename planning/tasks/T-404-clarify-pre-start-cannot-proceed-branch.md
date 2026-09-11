---
id: T-404-clarify-pre-start-cannot-proceed-branch
title: Clarify the cannot-proceed branch before start in autonomous skills
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies: []
updated_at: "2026-09-11T16:28:46Z"
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
