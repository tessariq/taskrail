---
id: T-160-ship-the-lifecycle-complete-task-implementation
title: Ship the lifecycle-complete task implementation prompt
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-implementation-prompt
dependencies:
    - T-158-bind-completion-and-verification-with-stable
    - T-159-add-a-versioned-workflow-prompt-catalog
    - T-168-parse-and-validate-an-optional-autonomous-run
    - T-217-release-interrupted-active-work-safely
updated_at: "2026-08-04T21:32:13Z"
---

# T-160-ship-the-lifecycle-complete-task-implementation Ship the lifecycle-complete task implementation prompt

## Description

Ship the built-in task implementation prompt and align full-task packaged skills
on one lifecycle-complete workflow. The prompt drives independent review and
verification while preserving Taskrail as orchestration rather than an LLM
provider.

## Acceptance

- Rendering uses only declared path/ID/version placeholders and instructs
  inspection of repository files; it never interpolates body contents or
  undeclared context.
- The workflow covers freshness before every writer, start, implementation,
  simplification, regression perturbation, fresh-context review/fallback
  labeling, correction of high/medium findings, and re-check before closure.
- Success completes then passes; cannot-proceed blocks then fails; deliberate
  rework may remain in progress with fail. Each branch checks writer exits, then
  creates one complete local commit containing implementation and generated
  task/state bytes; completed-unverified/audit recovery reruns only the
  safe step before commit.
- Every consumed command uses the common JSON result, and interrupted/manual
  rework guidance names direct-operator `task release` without allowing a
  delegated child to relinquish its selected task.
- Headless ambiguity, credentials, destructive scope, and barriers stop for a
  human; at most two real follow-ups are created without `loop_policy` or
  `loop_reason` and remain implicitly held.
- Delegated task implementation may use only its granted lifecycle and follow-up
  write sets. It cannot allow, hold, clear, or otherwise mutate task-local loop
  policy, and authoring a follow-up body cannot grant unattended authorization.
- Full-task skills use equivalent blocks, keep `autonomous-verify` as the separate
  post-transition workflow, and leave provider command, credentials, remote push,
  and sandboxing to callers.
- Prompt guidance inventories existing repository primitives, stops on material
  ambiguity, and traces requirements to observable executable evidence before
  implementation.

## Verification Notes

- Map each branch to golden/mutation fixtures proving lifecycle-before-commit,
  generated-byte inclusion, source guard, simplification, fresh review,
  perturbation, barriers, implicitly held follow-ups, delegated policy refusal,
  exits, and recovery.
- Manually render path-valued context and exercise success, blocked, rework, and
  partial-completion instructions without provider invocation by Taskrail.

## Implementation Notes
