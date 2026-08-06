---
id: T-239-edit-exact-id-dependencies-safely
title: Edit exact-ID dependencies safely
status: todo
priority: high
spec_ref: specs/v0.5.0.md#exact-id-dependency-editing
dependencies:
    - T-229-canonicalize-v0-5-lifecycle-and-task-identities
    - T-223-run-every-v0-5-command-against-local-storage
updated_at: "2026-08-06T13:46:30Z"
---

# T-239-edit-exact-id-dependencies-safely Edit exact-ID dependencies safely

## Description

Provide the sanctioned v0.5 apply path for accepted dependency-review findings
using exact full IDs and one-edge transactional changes.

## Acceptance

- A1. Add appends one edge and rejects missing, self, duplicate, cancelled, or
  cyclic dependencies; remove deletes exactly one existing edge and rejects absence.
- A2. Both commands accept live open targets, preserve dependency order and every
  non-target byte, reproject state, and expose exact preview/apply results.
- A3. Fuzzy identifiers, delegated loop children, and invalid local/committed
  storage candidates refuse without writes.

## Verification Notes

- A1: table-driven graph fixtures observe the one changed edge or stable error.
- A2: raw task/state diff and preview/apply golden evidence proves preservation.
- A3: identity/delegation/storage negatives retain identical repository snapshots.

## Implementation Notes
