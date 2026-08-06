---
id: T-172-enforce-autonomous-loop-lifecycle-and-delivery
title: Enforce autonomous loop lifecycle and delivery postflight
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-158-bind-completion-and-verification-with-stable
    - T-160-ship-the-lifecycle-complete-task-implementation
    - T-171-contain-and-pin-autonomous-loop-child-processes
    - T-217-release-interrupted-active-work-safely
updated_at: "2026-08-04T21:32:13Z"
---

# T-172-enforce-autonomous-loop-lifecycle-and-delivery Enforce autonomous loop lifecycle and delivery postflight

## Description

Classify every child termination against frozen lifecycle, verification, Git
delivery, task-local loop policy, mutation, prompt/executable, lock, and process
evidence. Apply committed- or local-mode delivery postconditions and continue only
after a delivered, fresh completed pass; semantic review convergence remains a
prompt/skill contract rather than a fabricated postflight attestation.

## Acceptance

- Every child termination maps by exact precedence to one specified per-child
  outcome; only completed_pass with zero child exit may continue. Invocation
  success terminates as `no_work` or `iteration_limit`, with no-work taking
  precedence when the last completed task exhausted eligible work.
- Fresh success requires a preflight-absent matching verification ID/path and
  completion ID across every surface; stale/audit/partial-complete/block/rework/
  no-progress/child failure stop with exact safe recovery.
- Completed audit failure requires a fresh bound pass followed by a distinct
  fresh failing audit in the same iteration; a completed fail without both
  artifact identities is invalid postflight. Diagnostics identify the
  intermediate pass verification/binding separately from the final fail.
- Delivered recognized outcomes require clean tree, same full attached ref,
  unchanged frozen spec/config/layout/storage/review/prompt/executable bytes,
  unchanged pre-existing `loop_policy` and `loop_reason` fields, valid task
  policy, and no contained process; remote is not_checked.
- Committed delivery requires implementation plus generated task/state bytes in
  a descendant local commit. Local completed-pass requires a descendant product
  commit; blocked/rework requires one only when product bytes changed, otherwise
  unchanged HEAD plus exact valid ignored lifecycle/verification bytes is valid.
  No local branch stages/commits Taskrail metadata or creates an empty delivery
  commit.
- Pre-existing non-selected task bytes and selected immutable content are
  protected; only canonical lifecycle fields/notes and at most two real
  follow-ups may change. Every new follow-up omits `loop_policy` and `loop_reason`
  and remains implicitly held; any child policy-field mutation is an integrity
  failure.
- Final diagnostics always report outcome, child exit/signal, all identity
  before/after values, validation, Git/ref/HEAD, task-local policy source and
  values, storage mode/root, configured/effective review maximum and source,
  prompt/executable hashes, mutation/process violations, local commits, remote,
  and next action.
- Exact dry-run and result-file nested schemas define nullability, source enums,
  iteration counting, commit/violation ordering, and preflight refusal behavior
  without fabricating iteration-only evidence.
- Optional `--result-file` atomically publishes the same final diagnostics in one
  common-envelope JSON document without mixing them with streamed child output;
  requested result publication failure is non-zero. Once its destination is
  valid, preflight refusals and handled interruptions also publish their exact
  error envelope when possible.
- Result destinations resolve from the caller's original cwd, must be absent
  canonical paths outside the worktree, Git directories, and managed inputs,
  traverse no symlink/reparse point, and publish no-clobber without invalidating
  reported cleanliness.
- Destination errors map exactly before launch and final publication rechecks the
  retained parent identity plus target absence before no-clobber creation.

## Verification Notes

- Map every outcome to setup/action/public diagnostic/filesystem+Git evidence;
  cover stale/mismatched IDs, every ref/ancestry/dirty/control/task mutation,
  bounded implicitly held follow-ups, frozen policy fields, iteration caps, and
  recovery-only verify.
- Persist manual success, hold, blocked, rework, partial-complete, audit-fail,
  external-writer, task-policy mutation, stale evidence,
  executable/prompt mutation, and containment-failure reports.

## Implementation Notes
