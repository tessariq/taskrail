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
updated_at: "2026-08-04T21:32:13Z"
---

# T-172-enforce-autonomous-loop-lifecycle-and-delivery Enforce autonomous loop lifecycle and delivery postflight

## Description

Classify every child termination against frozen lifecycle, verification, Git
delivery, policy, mutation, prompt/executable, lock, and process evidence.
Continue only after a fully delivered fresh completed pass.

## Acceptance

- Every termination maps to exactly one specified outcome; only completed_pass
  with zero child exit may continue, and iteration-cap success reports remaining
  work.
- Fresh success requires a preflight-absent matching verification ID/path and
  completion ID across every surface; stale/audit/partial-complete/block/rework/
  no-progress/child failure stop with exact safe recovery.
- Delivered recognized outcomes require clean tree, same full attached ref,
  descendant HEAD with a local commit, unchanged frozen spec/config/layout/
  prompt/executable/policy presence and bytes, valid policy, and no contained
  process; remote is not_checked.
- Pre-existing non-selected task bytes and selected immutable content are
  protected; only canonical lifecycle fields/notes and at most two real
  follow-ups with exact authorized policy insertion may change.
- Final diagnostics always report outcome, child exit/signal, all identity
  before/after values, validation, Git/ref/HEAD, policy, prompt/executable hashes,
  mutation/process violations, local commits, remote, and next action.

## Verification Notes

- Map every outcome to setup/action/public diagnostic/filesystem+Git evidence;
  cover stale/mismatched IDs, every ref/ancestry/dirty/control/task mutation,
  bounded follow-ups, policy insertion, iteration caps, and recovery-only verify.
- Persist manual success, hold, blocked, rework, partial-complete, audit-fail,
  external-writer, policy-presence mutation, stale evidence,
  executable/prompt mutation, and containment-failure reports.

## Implementation Notes
