---
id: T-272-implement-init-status-and-warning-machine
title: Implement init status and warning machine contracts
status: completed
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-213-define-the-uniform-agent-machine-api
    - T-214-bootstrap-and-migrate-human-owned-repository-notes
updated_at: "2026-08-12T10:09:13Z"
---

# T-272-implement-init-status-and-warning-machine Implement init status and warning machine contracts

## Description

Complete the init and status machine results and the closed warning union so agents
can discover storage, preview bootstrap or migration choices, and react to
advisories without reconstructing repository paths or parsing human diagnostics.

## Acceptance

- A1. Init preview/apply reports the exact common result shape for layout, config,
  writes, notes, skills, exclusions, continuation-note choices, and validation;
  refusals are error envelopes rather than result actions.
- A2. Status reports exact active storage mode, root, and resolved physical
  artifacts directory for committed and explicitly supplied local contexts without
  exposing an overlay path as a durable citation.
- A3. Every inherited and v0.5 warning uses its exact closed variant only in the
  envelope warning array, and warnings do not alter exit status.
- A4. Init reports empty skill/exclusion arrays when skills are not requested and
  deterministic committed/local inventories when they are requested.

## Verification Notes

- A1: compare fresh, current, migration-preview, migrated, and refusal outputs with
  strict init result/error goldens, including note choices supplied by T-214.
- A2: exercise committed and fixed-local storage contexts and verify exact roots
  and usable transient artifact paths.
- A3: trigger every warning variant and mutate cross-variant fields to demonstrate
  strict decoding and exit neutrality.
- A4: compare no-skills, committed-skills, local-skills, and pending-refresh
  inventories for exact deterministic paths and actions.

## Implementation Notes

- 2026-08-12T08:28:21Z: verification fail
- 2026-08-12T10:09:00Z: Implemented init/status/warning machine contracts; all required checks and the final fresh portability review passed.
- 2026-08-12T10:09:13Z: verification pass
