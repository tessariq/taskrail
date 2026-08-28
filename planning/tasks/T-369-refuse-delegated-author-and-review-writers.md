---
id: T-369-refuse-delegated-author-and-review-writers
title: Refuse delegated author and review writers consistently
status: completed
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-161-apply-reviewed-task-bodies-with-compare-and-swap
    - T-215-add-the-generic-review-artifact-publisher
updated_at: "2026-08-28T08:26:52Z"
completion_id: "5f5ad1aec93b9d94bc72151ba7f0b50c"
last_verification_id: "1bee71b62476869edf7766820b5a1ecf"
last_verification_result: pass
last_verified_at: "2026-08-28T08:26:52Z"
last_verified_completion_id: "5f5ad1aec93b9d94bc72151ba7f0b50c"
---

# T-369-refuse-delegated-author-and-review-writers Refuse delegated author and review writers consistently

## Description

Make task authoring and every review publisher report the canonical delegated
capability refusal when invoked by a loop child that lacks authority for those
write sets. Strict JSON and human modes must classify the same refusal.

## Acceptance

- `task author` and every `review publish` type refuse unauthorized delegation
  before semantic publication with `delegated_write_refused` and common details.
- Their exact schema-v1 error subsets and drift registries include that code;
  ordinary lock contention remains `lock_held` and direct operators are unchanged.
- Negative tests prove no proposal, destination, task, state, lock, or recovery
  bytes change on refusal in committed and local storage.

## Verification Notes

- Exercise valid and invalid delegation tokens against task author plus spec, task,
  decomposition, and workflow publishers in text and JSON modes.
- Run focused ownership and machine-contract tests, full tests, race tests, vet,
  and cross-platform CI.

## Implementation Notes

- 2026-08-28T08:26:40Z: Refused delegated task authoring and review publication before semantic reads or writes; aligned machine contracts and regression coverage.
- 2026-08-28T08:26:52Z: verification pass id 1bee71b62476869edf7766820b5a1ecf previous none completion 5f5ad1aec93b9d94bc72151ba7f0b50c
