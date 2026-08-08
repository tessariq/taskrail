---
id: T-171-contain-and-pin-autonomous-loop-child-processes
title: Pin autonomous loop writer ownership
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-308-publish-deterministic-loop-selection-and-dry-run
updated_at: "2026-08-04T21:32:13Z"
---

# T-171-contain-and-pin-autonomous-loop-child-processes Pin autonomous loop writer ownership

## Description

Own one continuous loop lock and one staged executable identity across an
invocation, and let only bounded delegated child writers join that ownership.
Generic process launch and prompt transport are owned by T-309; process-tree
containment is owned by T-243, T-310, and T-311.

## Acceptance

- Loop acquires the Git-common-directory writer lock before semantic preflight and
  retains the same ownership continuously through staging, every child and
  cleanup, postflight, between-iteration selection, terminal diagnostics, and
  final release. A concurrent writer or loop cannot observe an unlocked gap.
- Before the first child, the running executable is copied once to a hash-named
  no-replace regular file under the Git common directory and its bytes are
  SHA-256 verified. Every iteration uses that exact absolute path and digest;
  cleanup removes only this invocation's copy after all delegated writers exit.
- Lock metadata binds command, process/host/start, repository, storage mode,
  invocation ID, and delegation-token digest without exposing the token.
  Conflicting inherited `TASKRAIL`, `TASKRAIL_EXECUTABLE_SHA256`,
  `TASKRAIL_DELEGATION_ID`, or `TASKRAIL_DELEGATION_TOKEN` refuses before launch.
- A delegated writer joins only after proving the repository, frozen storage
  mode/root, invocation and selected task, staged executable bytes, secret token,
  command capability, and exact field/write-set bounds. It may perform selected
  lifecycle writes and verification-created implicit-hold follow-ups only.
- Delegated task-policy mutation, unrelated task creation or writes, repository
  review-policy change, storage mismatch, and review-budget widening are refused
  without releasing or replacing the loop's lock.

## Verification Notes

- Concurrency helpers attempt unrelated writes before launch, during a child,
  after child exit, during postflight, and between iterations, proving one lock
  identity remains continuously held.
- Staging fixtures cover no-replace collisions, byte/hash changes, multi-iteration
  reuse, conflicting inherited identity variables, and cleanup only after all
  joined writers terminate.
- Exercise every allowed and refused delegated capability in committed and local
  mode, including wrong repository/storage/task/executable/token/budget and
  selected-task policy mutation, while inspecting lock metadata for secret
  non-disclosure.

## Implementation Notes
