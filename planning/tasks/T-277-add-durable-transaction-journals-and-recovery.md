---
id: T-277-add-durable-transaction-journals-and-recovery
title: Add durable transaction journals and recovery phases
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
    - T-222-initialize-and-discover-ignored-local-taskrail
    - T-317-bind-delegated-grants-to-the-owner-s-declared-task
    - T-322-provide-handle-bound-durable-filesystem-primitives
    - T-323-enforce-repository-recovery-fences-and-stable
updated_at: "2026-08-14T16:05:09Z"
---

# T-277-add-durable-transaction-journals-and-recovery Add durable transaction journals and recovery phases

## Description

Extend normal snapshot transactions with durable journals and explicit publication
phases so an interrupted semantic write leaves a complete recovery fence from
which the shared recovery command can choose one mechanical safe action. Consume
the canonical repository/runtime context, delegated write-set binding, durable
filesystem primitives, and repository-wide admission fence delivered by the
prerequisites; this task owns the durable journal state machine and recovery
engine, not those substrate concerns or public `taskrail recover` command wiring.

## Acceptance

- A1. Before the first semantic publication, a durable transaction records exact
  original presence/bytes/modes, candidate digests, path identities, owning
  command, and transaction identity beneath the T-222 shared lock root using
  T-322 durable publication primitives.
- A2. Durable phase transitions are persisted in canonical order and an
  interruption leaves transaction state that T-323 admits only as a
  repository-wide `recovery_pending` fence.
- A3. Journal state determines exactly one safe `restore_original`,
  `accept_candidate`, or `clear_fence` outcome without Git reset or semantic
  inference; decisions use whole-set presence/bytes/modes and rebound identity
  evidence, and unexpected state refuses without overwrite.
- A4. Recovery is interruption-safe, compare-and-swap protected, and clears the
  fence only after a lock-bound final whole-set CAS and the selected action reaches
  a durably persisted, valid complete state. Public command routing remains T-232.

## Verification Notes

- A1: inspect a prepared mixed-path journal and compare every recorded identity,
  original, mode, candidate digest, and command with the frozen transaction.
- A2: interrupt each durable phase and observe the retained phase plus blocked
  reads/writes.
- A3: construct restore, accept, clear, and unexpected-byte states and compare the
  single previewed action and no-clobber behavior.
- A4: interrupt each recovery action and retry it, observing either the same safe
  action or a completed valid state with no lost external edits.
- Integration: exercise T-317 bounded delegated write sets, T-322 failure
  injection, and T-323 admission before, during, and after every journal phase.

## Implementation Notes

- 2026-08-12T20:37:45Z: Correctness review found the proposed portable path-based journal cannot satisfy A1-A4: preparation can strand an unrecoverable fence, recovery lacks lock-bound final CAS, path identity is not no-follow/handle-bound, retained fences do not block normal readers/writers, and post-rename fsync failure can lose recovery evidence. Operator must approve decomposition around a handle-bound filesystem primitive plus global fence integration, or revise the portability contract.
- 2026-08-12T20:37:59Z: verification fail
- 2026-08-14T16:05:09Z: Operator approved the portable concurrency contract and completed T-222 repository discovery, T-317 delegated grant binding, T-322 durable filesystem primitives, and T-323 repository-wide recovery admission; T-277 now owns only journal phases and the lock-bound recovery engine.
