---
id: T-323-enforce-repository-recovery-fences-and-stable
title: Enforce repository recovery fences and stable semantic reads
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-322-provide-handle-bound-durable-filesystem-primitives
updated_at: "2026-08-14T15:33:17Z"
---

# T-323-enforce-repository-recovery-fences-and-stable Enforce repository recovery fences and stable semantic reads

## Description

Make retained or malformed transaction state a repository-wide admission fence.
All semantic reads and writes must resolve one canonical repository/runtime root,
inspect recovery state through stable handle-bound reads, and return the common
`recovery_pending` refusal before exposing or mutating possibly incoherent state.
This task owns admission and stable inspection, not journal phase transitions or
the recovery action engine.

## Acceptance

- A1. One canonical recovery root is derived from the T-222 repository context;
  linked worktrees share it and decoy logical, worktree-local, or aliased roots
  cannot bypass it.
- A2. Every semantic command family enters through a common admission check before
  managed reads or writes; any retained transaction returns `recovery_pending`
  with canonical transaction evidence and no partial semantic result.
- A3. Missing, malformed, noncanonical, linked, special, identity-changing, or
  concurrently replaced transaction state fails closed as `recovery_pending` and
  is never ignored, repaired, or cleared by admission.
- A4. Admission obtains a stable snapshot of recovery membership and bytes using
  T-322 primitives, detects changes through the semantic operation boundary, and
  prevents a reader from observing a mixed pre/post-transaction view.
- A5. Read-only and writer races, linked worktrees, committed/local storage, and
  non-Git committed repositories all use the same fence policy; commands that
  cannot establish the required stable repository identity refuse safely.

## Verification Notes

- A1: exercise ordinary repositories, linked worktrees, local mode, descendant
  invocation, and decoy recovery roots against one canonical identity.
- A2: table-test every command family with retained valid recovery state and prove
  no semantic read result or tracked write occurs.
- A3-A4: race creation, replacement, mutation, aliasing, and malformed entries at
  every inspection boundary and assert deterministic fail-closed behavior.
- A5: run representative CLI sandbox flows for read-only and writer commands in
  Git committed/local and non-Git committed repositories.

## Implementation Notes

- 2026-08-14T15:33:11Z: Implemented canonical repository recovery admission with stable no-follow transaction inspection, strict recovery_pending machine evidence, command-family fencing, race coverage, and Git/non-Git sandbox verification.
- 2026-08-14T15:33:17Z: verification pass
