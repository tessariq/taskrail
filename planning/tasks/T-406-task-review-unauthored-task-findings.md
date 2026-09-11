---
id: T-406-task-review-unauthored-task-findings
title: Stop task review from publishing empty findings for unauthored tasks
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#existing-task-review
dependencies: []
updated_at: "2026-09-11T16:28:47Z"
---

# T-406-task-review-unauthored-task-findings Stop task review from publishing empty findings for unauthored tasks

## Description

In `taskrail-task-review-committed` the v0.5.0 candidate published a review with
an empty findings array for a scaffold whose description, acceptance, and
verification are all TODO, while the local arm on the same task published a
high-severity finding. The skill requires judging outcome alignment, sizing,
acceptance, and oracles, but does not stop an empty review of unauthored work.

Outcome: reviewing a task with placeholder sections cannot publish as clean.

## Acceptance

- The skill requires a finding, or an explicit refusal, when the reviewed
  task's outcome, acceptance, or verification is placeholder text.
- Whether `review publish` should also reject such a bundle is decided with the
  maintainer and recorded here.

## Verification Notes

- Skill contract test; eval case assertion.

## Implementation Notes
