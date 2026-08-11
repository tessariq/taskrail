---
id: T-156-protect-existing-semantic-writers-with-snapshot
title: Protect normal writes with snapshot transactions
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-155-add-the-repository-mutation-lock-protocol
    - T-230-enforce-the-normative-v0-5-machine-schema
updated_at: "2026-08-11T08:46:54Z"
---

# T-156-protect-existing-semantic-writers-with-snapshot Protect normal writes with snapshot transactions

## Description

Provide the normal transaction contract used by v0.5 semantic writers: under the
shared lock, protect the complete consumed and published snapshot, validate the
candidate before publication, and preserve concurrent external edits on handled
failure. Delegated work receives only an explicitly narrowed write capability.

## Acceptance

- A1. A normal transaction snapshots the complete consumed/published set, validates
  the complete candidate before the first write, and commits atomically replaced
  files while holding the repository mutation lock.
- A2. A handled multi-file failure restores only paths still equal to this
  transaction's candidates and reports conflicts or rollback failure without
  claiming crash atomicity.
- A3. Error observations expose deterministic typed snapshots for managed,
  worktree, and Git paths with exact original, candidate, and current digests.
- A4. A delegated capability remains bound to its repository, storage, command,
  selected task, executable, and permitted field/write set and cannot be widened
  by joining or nesting work.

## Verification Notes

- A1: exercise successful single- and multi-file writes plus candidate-validation
  failure and observe all-or-none managed bytes under the lock.
- A2: induce handled publication failure and rollback races, then inspect preserved
  external edits and exact conflict/rollback outcomes.
- A3: compare mixed managed/worktree/Git error snapshots and deterministic digest
  ordering against exact preflight/candidate/current bytes.
- A4: attempt delegated widening across each bound dimension and observe refusal
  while permitted narrowed work succeeds.

## Implementation Notes

- 2026-08-11T08:46:19Z: Added internal/repotx: the normal transaction contract v0.5 semantic writers publish through. One transaction snapshots the complete consumed/published set under the repository mutation lock, validates the complete candidate before the first write, compare-and-swaps the whole set and again per path immediately before each write, and atomically replaces each published file. A handled failure rolls back only paths still equal to this transaction's candidates, preserving and naming external edits as rollback failure without claiming crash atomicity. Failures are typed kinds carrying deterministic managed/worktree/Git snapshots with exact original/candidate/current digests, mapped to registered v0.5 error codes. repolock.Capability gained selected-task and write-set bounds that only ever narrow, and repotx refuses a delegated ownership that arrives unbounded on either.
- 2026-08-11T08:46:54Z: verification pass
