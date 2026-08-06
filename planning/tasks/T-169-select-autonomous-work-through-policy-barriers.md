---
id: T-169-select-autonomous-work-through-policy-barriers
title: Select autonomous work through task-local loop policy
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-local-loop-policy
dependencies:
    - T-168-parse-and-validate-an-optional-autonomous-run
updated_at: "2026-08-04T21:32:13Z"
---

# T-169-select-autonomous-work-through-policy-barriers Select autonomous work through task-local loop policy

## Description

Define deterministic unattended selection by composing the existing read-only
active-spec eligibility and ranking with each task's local loop authorization.
Authorization changes selection only; lifecycle and dependency truth remain in
the task ledger.

## Acceptance

- Selection starts with the same active-spec eligibility, dependency handling,
  priority, and ID tie-breaks as read-only status; it introduces no second queue
  or ordering source.
- Candidates are exactly active-spec `todo` tasks with explicit `allow` and normal
  read-only eligibility. Among candidates, priority, dependency, and full-task-ID
  tie-breaks exactly match status selection; policy location, mutation time, and
  list order never affect ranking.
- Explicitly or implicitly held tasks are bypassed for unrelated work. A held task
  in an allowed candidate's unresolved transitive dependency closure blocks only
  that candidate and its dependents and appears in ordered `held_dependencies`.
- Allowed tasks that are blocked, off-spec, terminal, in progress, or waiting on
  dependencies do not launch, but selection continues to an independently
  eligible allowed task. If none exists, action is clean `none` with dispositions;
  malformed policy or repository inconsistency is non-zero.
- Selection is read-only and never adds, clears, or changes task-local loop-policy
  fields. Newly created tasks and follow-ups therefore remain implicit holds.
- Text and JSON expose the selected candidate and exact task-row fields
  `task_id`, `status`, `active_spec`, `source`, `effective_policy`, `reason`,
  `eligible`, `held_dependencies`, and `disposition`, plus action, violations,
  and the result reason in stable order. JSON uses the common envelope with
  non-null top-level warnings and non-null result violations; repeated selection
  over unchanged bytes is identical and no additional public ranking-input field
  is introduced.

## Verification Notes

- Map explicit allow, explicit hold, implicit hold, no-work, blocked, in-progress,
  malformed, held-dependency, ordinary dependency, and off-spec cases to action,
  exit code, and no-launch evidence.
- Use mixed-priority/dependency fixtures to prove status-equivalent ranking,
  dependency-local hold behavior, stable JSON, and byte-identical repeated
  selection.

## Implementation Notes
