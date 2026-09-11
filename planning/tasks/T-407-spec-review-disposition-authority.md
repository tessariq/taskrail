---
id: T-407-spec-review-disposition-authority
title: Keep spec-review agents from authoring dispositions or rerunning lenses
status: todo
priority: high
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies: []
updated_at: "2026-09-11T16:28:48Z"
---

# T-407-spec-review-disposition-authority Keep spec-review agents from authoring dispositions or rerunning lenses

## Description

`taskrail-spec-review` says "a human alone decides dispositions" and "Unchanged
exact bytes never justify another lens round". In the v0.5.0 skill evaluation:

- `taskrail-spec-review-local`: the agent itself recorded `rejected`
  dispositions for all four findings plus an `approved_at` time, and its final
  account said only that all findings were rejected.
- `taskrail-spec-review-committed`: the agent ran five lens rounds against
  unchanged spec bytes, discarded a round that had findings over its own
  manifest error, announced a no-finding round, and published a bundle with no
  findings. It also edited `.git/info/exclude` (root cause T-398).

Mechanical grading passed both; only transcript review caught this.

Outcome: an agent cannot author human dispositions or re-run lenses toward a
preferred result without that being refused or visible in the published bundle.

## Acceptance

- Publishing a disposition requires evidence the agent cannot fabricate, or the
  bundle marks agent-recorded dispositions; mechanism decided with the
  maintainer.
- Repeated lens rounds over identical spec bytes are refused or recorded in the
  published bundle.
- The spec-review eval cases assert both.

## Verification Notes

- Publisher tests for disposition provenance and repeated rounds; eval case
  assertions.

## Implementation Notes
