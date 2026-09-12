---
id: T-410-plant-local-eval-decoys
title: Plant local skill-eval decoys at logical paths and validate them
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-09-11T16:28:50Z"
---

# T-410-plant-local-eval-decoys Plant local skill-eval decoys at logical paths and validate them

## Description

Local-mode eval fixtures plant their decoy at `decoy/planning/STATE.md`, not at
a logical managed path, so an agent that guesses logical paths finds nothing and
the decoy never tempts it. `seed.json` records decoy contents that differ from
the planted bytes, and seed validation only checks that the fields are non-empty.

Outcome: decoys sit where a path-deriving agent would read, and seeds are
checked against the planted fixture.

## Acceptance

- Each local fixture plants its decoy at the logical path a storage-deriving
  agent would open.
- Seed validation fails when the recorded decoy path or contents disagree with
  the fixture.

## Verification Notes

- Registry validation tests.

## Implementation Notes
