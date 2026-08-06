---
id: T-156-protect-existing-semantic-writers-with-snapshot
title: Build the shared snapshot transaction substrate
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-155-add-the-repository-mutation-lock-protocol
    - T-230-enforce-the-normative-v0-5-machine-schema
updated_at: "2026-08-06T13:52:16Z"
---

# T-156-protect-existing-semantic-writers-with-snapshot Build the shared snapshot transaction substrate

## Description

Build reusable normal and durable transaction primitives over the shared lock:
complete read/write snapshots, candidate staging and validation, atomic file and
directory publication, compare-and-swap rollback, retained recovery metadata, and
scoped capabilities. Writer-family integration is owned by T-233 and T-234.

## Acceptance

- Normal transactions snapshot the complete consumed/published set, validate
  candidates before writes, and roll back handled failures without claiming crash
  atomicity.
- Durable transactions persist exact originals, candidates, path identities, and
  phases before publication so shared recovery can restore, accept, or clear
  without inference.
- File replacement, absent-directory commit, fsync boundaries, external-edit
  preservation, and rollback failure produce the normative snapshots and recovery
  reference.
- Capability objects bind repository, storage, command, selected task, executable,
  and field/write set and cannot be widened by a delegated join.

## Verification Notes

- Exercise normal and durable transaction helpers against changed bytes, absent
  paths, no-clobber directories, interruption, rollback races, and fsync faults.
- Prove capability narrowing and exact snapshots independently of command-specific
  writer semantics.

## Implementation Notes
