---
id: T-215-add-the-generic-review-artifact-publisher
title: Add the generic review artifact publisher
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-159-add-a-versioned-workflow-prompt-catalog
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-05T20:24:17Z"
---

# T-215-add-the-generic-review-artifact-publisher Add the generic review artifact publisher

## Description

Add one schema-aware publication command for spec, task, decomposition, and
workflow review proposals so static skills do not pretend to provide portable
atomic/no-follow filesystem guarantees.

## Acceptance

- `review publish --type spec|task|decomposition|workflow` implements exact per-type flags, schemas, common JSON envelopes, and dry-run/apply parity.
- Every type rechecks subject/session/digest/path bindings, caps, expected snapshots, and complete output sets before joining the shared writer/recovery protocol.
- Publication is canonical, no-follow, no-alias, no-clobber, and all-or-none. Spec/task/decomposition subjects stay unchanged; workflow retains report creation plus index CAS replacement.
- Capabilities exclude lifecycle, loop policy, spec activation, import apply, and verification. Conflict/interruption reports exact recovery without partial final files.
- Review skills hand untrusted proposals to this command and never write final artifacts directly.

## Verification Notes

- Map every type to strict schema/path/digest/session fixtures, preview snapshots, publication races, alias swaps, interruption points, rollback, and forbidden-write sentinels.
- Run simultaneous workflow/index and absent-destination attempts on Linux, macOS, and Windows and prove one complete outcome or none.

## Implementation Notes
