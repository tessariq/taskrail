---
id: T-298-bind-task-review-publication-to-resolved-prompts
title: Bind task review publication to resolved prompts
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
    - T-215-add-the-generic-review-artifact-publisher
updated_at: "2026-08-25T19:05:34Z"
completion_id: "8333eb649132d40996387de50493e716"
last_verification_id: "34f98b9c0e9c8d55a02a20e9fc3f9c24"
last_verification_result: pass
last_verified_at: "2026-08-25T19:05:34Z"
last_verified_completion_id: "8333eb649132d40996387de50493e716"
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

- 2026-08-25T19:05:25Z: Added commit-stage task-review prompt binding coverage for exact fields and all bound snapshot inputs.
- 2026-08-25T19:05:34Z: verification pass id 34f98b9c0e9c8d55a02a20e9fc3f9c24 previous none completion 8333eb649132d40996387de50493e716
