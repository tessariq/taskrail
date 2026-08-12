---
id: T-157-upgrade-repositories-transactionally-to-layout-2
title: Construct strict layout 2 migration candidates
status: todo
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-168-parse-and-validate-an-optional-autonomous-run
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-214-bootstrap-and-migrate-human-owned-repository-notes
updated_at: "2026-08-04T21:32:13Z"
---

# T-157-upgrade-repositories-transactionally-to-layout-2 Construct strict layout 2 migration candidates

## Description

Define the strict layout-2, state-schema-2, and task-policy compatibility boundary
and construct a complete validated migration candidate from supported layout-1
bytes. Candidate construction is write-free and preserves all meaning that later
preview decisions or durable apply must publish.

## Acceptance

- A1. Strict layout decoding accepts only layout 1 or the exact final/fenced
  layout-2 marker shapes; layout 2 requires explicit committed/local storage and a
  broad review-round maximum from 1 through 2, and unknown or invalid fields are
  never dropped.
- A2. Strict state-schema-2 decoding accepts only the bounded snapshot fields and
  valid optional verification tuple, rejects `continuation_notes` and rendered
  Notes, and never invents verification identity while migrating schema 1.
- A3. Strict task decoding accepts only absent or valid paired `loop_policy` and
  `loop_reason`, preserving existing pairs exactly and treating absence as implicit
  hold without granting unattended authority.
- A4. Supported layout-1 input constructs a complete layout-2 candidate with
  committed mode, broad review-round maximum 2, schema-2 state, preserved task
  meaning, and classified skill/note outcomes without writing repository bytes.
- A5. Invalid source marker, state, task policy, skill marker/parity, note
  destination, or configured legacy-policy path prevents candidate construction
  without silently repairing or discarding input.

## Verification Notes

- A1: decode valid layout-1/final/fenced layout-2 markers and mutate every field,
  enum, bound, omission, and fence relationship.
- A2: exercise fresh, migrated legacy, canonical verification, partial tuple,
  unknown-field, and reintroduced-note state fixtures.
- A3: decode absent and paired task-policy fields plus incomplete, invalid, and
  authority-granting migration cases; compare preserved task meaning.
- A4: construct a complete candidate from representative legacy repositories and
  compare every candidate byte/path while proving the source tree is unchanged.
- A5: perturb each source class independently, including exact `AUTONOMY.tsv`
  aliases and same-basename decoys, and observe deterministic refusal.

## Implementation Notes
