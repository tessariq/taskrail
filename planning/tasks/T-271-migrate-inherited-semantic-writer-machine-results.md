---
id: T-271-migrate-inherited-semantic-writer-machine-results
title: Migrate inherited semantic writer machine results
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-08T14:23:08Z"
---

# T-271-migrate-inherited-semantic-writer-machine-results Migrate inherited semantic writer machine results

## Description

Give inherited semantic writers one uniform machine contract, including JSON
support for lifecycle transitions, without changing their human-oriented behavior
or treating refusals and partial outcomes as successful results.

## Acceptance

- A1. Existing JSON-capable semantic writers return their exact registered v0.5
  result payloads inside the common envelope for preview and apply outcomes.
- A2. `start`, `complete`, and `block` support `--json` with the exact lifecycle
  result fields while retaining existing human text behavior.
- A3. Writer refusals, validation failures, conflicts, partial writes, rollback
  failures, and recovery requirements emit registered error envelopes with
  `applied` meaning the complete semantic operation committed.
- A4. Successful machine-mode writes preserve the same persisted lifecycle and
  validation meaning as equivalent human-mode writes.

## Verification Notes

- A1: exercise preview/apply success for each inherited writer family and compare
  strict result goldens.
- A2: run lifecycle success transitions in text and JSON sandboxes and compare
  persisted outcomes and exact machine fields.
- A3: induce each material refusal/publication boundary and inspect error code,
  `applied`, snapshots, and recovery details.
- A4: compare resulting task/state bytes and validation observations across text
  and JSON modes.

## Implementation Notes
