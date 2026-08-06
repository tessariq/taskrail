---
id: T-160-ship-the-lifecycle-complete-task-implementation
title: Ship the lifecycle-complete task implementation prompt
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-implementation-prompt
dependencies:
    - T-217-release-interrupted-active-work-safely
    - T-241-warn-on-passing-verification-before-completion
    - T-250-render-prompts-from-storage-neutral-context
updated_at: "2026-08-04T21:32:13Z"
---

# T-160-ship-the-lifecycle-complete-task-implementation Ship the lifecycle-complete task implementation prompt

## Description

Ship the built-in task implementation prompt with bounded independent review,
simplification, finding dispositions, lifecycle/delivery branches, and recovery
guidance. Packaged full-task skill command execution is owned by T-242.

## Acceptance

- Rendering uses only declared path/ID/version placeholders and instructs
  inspection of repository files; it never interpolates body contents or
  undeclared context.
- The workflow covers freshness before every writer, start, implementation,
  simplification, regression perturbation, fresh-context review/fallback
  labeling, bounded iterative review, correction of every current-scope finding,
  and re-check/re-review of materially changed final bytes before closure.
- Every finding receives `fix-now`, `separate-followup`, `blocked`, or `rejected`
  plus rationale. Budget exhaustion cannot defer current work or permit pass;
  clean review stops early, while final-pass material change remains rework.
- Success completes then passes; cannot-proceed blocks then fails; deliberate
  rework may remain in progress with fail. Each branch checks writer exits, then
  follows the selected storage-mode delivery contract: committed mode creates one
  commit containing implementation and generated task/state bytes; local success
  commits product bytes only, while metadata-only local blocked/rework may retain
  HEAD and never fabricates an empty commit. Completed-unverified/audit recovery
  reruns only the safe step before any required commit.
- Every consumed command uses the common JSON result, and interrupted/manual
  rework guidance names direct-operator `task release` without allowing a
  delegated child to relinquish its selected task.
- Headless ambiguity, credentials, destructive scope, and barriers stop for a
  human; necessary independently meaningful out-of-scope follow-ups are created
  only through selected-task verification, with no arbitrary numeric cap, no
  `loop_policy`/`loop_reason`, and no current-task repair deferred into follow-up.
- Delegated task implementation may use only its granted lifecycle and follow-up
  write sets. It cannot allow, hold, clear, or otherwise mutate task-local loop
  policy, and authoring a follow-up body cannot grant unattended authorization.
- The prompt leaves provider command, credentials, remote push, and sandboxing to
  callers and never claims reviewer identity attestation.
- Prompt guidance inventories existing repository primitives, stops on material
  ambiguity, and traces requirements to observable executable evidence before
  implementation.
- Committed mode delivers implementation plus generated planning bytes; local
  mode commits visible product changes only and leaves a valid ignored Taskrail
  lifecycle/verification outcome.

## Verification Notes

- Map each branch to golden/mutation fixtures proving lifecycle-before-commit,
  generated-byte inclusion, source guard, simplification, fresh review,
  perturbation, barriers, implicitly held follow-ups, delegated policy refusal,
  exits, and recovery.
- Manually render path-valued context and exercise success, blocked, rework, and
  partial-completion instructions without provider invocation by Taskrail.

## Implementation Notes
