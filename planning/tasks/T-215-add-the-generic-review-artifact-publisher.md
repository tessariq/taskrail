---
id: T-215-add-the-generic-review-artifact-publisher
title: Add the generic review artifact publisher
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-240-implement-the-normative-review-schema-decoders
updated_at: "2026-08-05T20:24:17Z"
---

# T-215-add-the-generic-review-artifact-publisher Add the generic review artifact publisher

## Description

Add the common no-follow publication core plus spec, task, and decomposition
directory adapters and storage-neutral review reads. Workflow report/index
derivation and pair publication remain T-166.

## Acceptance

- `review publish --type spec|task|decomposition` implements exact flags, fixed
  proposal bundles, 1 MiB per-file limits, final subtrees, common envelopes, and
  dry-run/apply parity through registered strict adapters.
- Every type rechecks subject/session/digest/path bindings, expected snapshots,
  manifest authority, adapter-registered consumed inputs and validators, and
  complete output sets before one no-clobber directory commit. T-255 registers
  prompt-template resolution after contextual rendering exists.
- Publication is canonical, no-follow, no-alias, no-clobber, and does not change
  reviewed subjects. Common routing admits the workflow adapter later without
  owning its report/index semantics.
- `review show` reads durable logical review paths through either storage mode;
  local publication requires explicit local initialization before proposal
  staging and never bootstraps from a pre-existing proposal.
- Capabilities exclude lifecycle, loop policy, spec activation, import apply, and
  verification. Directory publication either exposes one complete destination or none.
- Review skills hand untrusted proposals to this command and never write final artifacts directly.

## Verification Notes

- Map each owned type to strict schema/path/digest/session fixtures, preview
  snapshots, an injected consumed-input validator, publication races, alias swaps,
  interruption points, and forbidden-write sentinels.

## Implementation Notes
