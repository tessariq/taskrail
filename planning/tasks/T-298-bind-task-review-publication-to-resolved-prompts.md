---
id: T-298-bind-task-review-publication-to-resolved-prompts
title: Bind task review publication to resolved prompts
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
    - T-215-add-the-generic-review-artifact-publisher
updated_at: "2026-08-08T14:23:08Z"
---

# T-298-bind-task-review-publication-to-resolved-prompts Bind task review publication to resolved prompts

## Description

Attach the reusable prompt-binding hook to task-review publication so each durable
observation is bound to the current role-mandated task-review template.

## Acceptance

- A1. The task adapter requires prompt ID `task-review`, contract `v1`, exact
  template digest, and effective `builtin|replacement` source in `review.json`.
- A2. Strict binding shape/role validation precedes active resolution; invalid
  replacement and stale source/template use the specified error precedence.
- A3. Preview and apply snapshot and recheck prompt/config bytes with task/spec and
  proposal inputs immediately before the directory commit; drift publishes nothing.
- A4. Later prompt changes do not rewrite or invalidate published task-review
  history, and the binding makes no reviewer-delivery or identity claim.

## Verification Notes

- A1-A3: built-in and committed/local replacement matrices mutate every binding
  field, source class, template/config byte, and final recheck timing while
  asserting no destination on refusal.
- A4: historical `review show` after prompt changes returns unchanged bytes; wording
  assertions reject delivery, independence, and certification claims.

## Implementation Notes
