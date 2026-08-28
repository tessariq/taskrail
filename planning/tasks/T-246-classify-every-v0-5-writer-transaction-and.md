---
id: T-246-classify-every-v0-5-writer-transaction-and
title: Classify every v0.5 writer transaction and exception
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-291-prove-inherited-command-parity-in-local-storage
    - T-231-inspect-and-clear-stale-repository-locks-safely
    - T-244-publish-streamed-loop-results-out-of-band
    - T-298-bind-task-review-publication-to-resolved-prompts
    - T-299-bind-spec-review-publication-to-resolved-prompts
    - T-300-bind-decomposition-publication-to-resolved-prompts
    - T-305-publish-workflow-review-reports-and-memory
    - T-315-promote-local-packaged-skills-with-explicit
updated_at: "2026-08-28T11:10:54Z"
completion_id: "982cd67b08ec391fafea8cc62e4e3a28"
last_verification_id: "e7513b395bedcac4a7ce17d5d5db40c2"
last_verification_result: pass
last_verified_at: "2026-08-28T11:10:54Z"
last_verified_completion_id: "982cd67b08ec391fafea8cc62e4e3a28"
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

- 2026-08-28T11:10:41Z: Added an executable writer transaction matrix with publication-entrypoint drift checks.
- 2026-08-28T11:10:54Z: verification pass id e7513b395bedcac4a7ce17d5d5db40c2 previous none completion 982cd67b08ec391fafea8cc62e4e3a28
