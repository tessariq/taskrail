---
id: T-246-classify-every-v0-5-writer-transaction-and
title: Classify every v0.5 writer transaction and exception
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-224-promote-local-taskrail-state-into-committed
    - T-244-publish-streamed-loop-results-out-of-band
    - T-245-cover-the-complete-implicit-local-bootstrap-matrix
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
updated_at: "2026-08-06T13:46:30Z"
---

# T-246-classify-every-v0-5-writer-transaction-and Classify every v0.5 writer transaction and exception

## Description

Close the writer registry after every feature exists so each mutation is classified
as a normal transaction, named durable flow, single-directory commit, streamed
result publication, or an explicit non-transactional/read-only surface.

## Acceptance

- A1. One implementation-derived matrix contains every command writer, storage
  mode, lock capability, consumed/published set including review prompt/config
  snapshots, durability class, and recovery owner.
- A2. Unknown writers, duplicate ownership, unclassified filesystem sinks, or a
  command whose implementation disagrees with its normative class fail checks.
- A3. The matrix confirms normal writers do not claim crash atomicity and durable
  flows cannot publish without recoverable phase evidence.

## Verification Notes

- A1: registry output is compared with the spec/command/schema inventories.
- A2: mutation tests add an unregistered sink, omit a publisher prompt input, and
  apply a wrong durability annotation, observing deterministic failure.
- A3: cross-flow interruption fixtures demonstrate the promised boundary for each class.

## Implementation Notes
