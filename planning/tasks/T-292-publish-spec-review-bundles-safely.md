---
id: T-292-publish-spec-review-bundles-safely
title: Publish spec review bundles safely
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-215-add-the-generic-review-artifact-publisher
updated_at: "2026-08-08T14:23:08Z"
---

# T-292-publish-spec-review-bundles-safely Publish spec review bundles safely

## Description

Add the spec-review adapter to the shared directory publisher so one approved
four-lens bundle becomes immutable durable evidence without editing the spec.

## Acceptance

- A1. `review publish --type spec` accepts only the declared spec flags and the
  exact five-file proposal inventory beneath the active transient proposal root.
- A2. The strict manifest is the sole authority for lens membership, order,
  digests, dispositions, session, selected spec path/digest, and final destination;
  every finding is covered and no high/medium finding remains unresolved.
- A3. Preview and apply recheck exact spec/config/proposal snapshots, session and
  version path identities, no-follow boundaries, and destination absence before
  publishing the five proposal files byte-for-byte through one directory commit.
- A4. Any malformed, stale, aliased, tracked/staged/non-ignored, or changed input
  leaves the spec and final review subtree unchanged. Prompt binding is T-299.

## Verification Notes

- A1/A2: positive and mutation bundles cover fixed inventory/order, every
  disposition, duplicate/missing findings, digest/session/version mismatches, and
  per-file caps.
- A3/A4: dry-run/apply parity, subject/config races, alias swaps, Git visibility,
  and commit fault injection prove exact-byte all-or-none publication.

## Implementation Notes
