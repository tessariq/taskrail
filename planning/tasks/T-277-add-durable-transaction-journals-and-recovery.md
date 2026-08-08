---
id: T-277-add-durable-transaction-journals-and-recovery
title: Add durable transaction journals and recovery phases
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
updated_at: "2026-08-08T14:23:08Z"
---

# T-277-add-durable-transaction-journals-and-recovery Add durable transaction journals and recovery phases

## Description

Extend normal snapshot transactions with durable journals and explicit publication
phases so an interrupted semantic write leaves a complete recovery fence from
which the shared recovery command can choose one mechanical safe action.

## Acceptance

- A1. Before the first semantic publication, a durable transaction records exact
  original presence/bytes/modes, candidate digests, path identities, owning
  command, and transaction identity beneath the shared lock root.
- A2. Durable phase transitions are persisted in canonical order and an
  interruption leaves a recovery fence that blocks affected reads and writes.
- A3. Journal state determines exactly one safe `restore_original`,
  `accept_candidate`, or `clear_fence` outcome without Git reset or semantic
  inference; unexpected bytes refuse without overwrite.
- A4. Recovery is interruption-safe, compare-and-swap protected, and clears the
  fence only after the selected action reaches a valid complete state.

## Verification Notes

- A1: inspect a prepared mixed-path journal and compare every recorded identity,
  original, mode, candidate digest, and command with the frozen transaction.
- A2: interrupt each durable phase and observe the retained phase plus blocked
  reads/writes.
- A3: construct restore, accept, clear, and unexpected-byte states and compare the
  single previewed action and no-clobber behavior.
- A4: interrupt each recovery action and retry it, observing either the same safe
  action or a completed valid state with no lost external edits.

## Implementation Notes
