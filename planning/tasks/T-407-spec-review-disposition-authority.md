---
id: T-407-spec-review-disposition-authority
title: Keep spec-review agents from authoring dispositions or rerunning lenses
status: completed
priority: high
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies: []
updated_at: "2026-09-14T18:23:13Z"
completion_id: "61ea2251870cb8c223513c3362d60739"
last_verification_id: "f91a011d479bbd63f87a37808bfde379"
last_verification_result: pass
last_verified_at: "2026-09-14T18:23:13Z"
last_verified_completion_id: "61ea2251870cb8c223513c3362d60739"
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

- 2026-09-14T12:15:32Z: verification fail id e8c1c56b9492d2869d6839d50882629e previous none completion none
- 2026-09-14T18:23:13Z: Publish declared spec-review round history with explicitly unverified dispositions; require schema-2 evidence for new decomposition while retaining historical reads. Independent review findings fixed and sandbox CLI checks pass.
- 2026-09-14T18:23:13Z: verification pass id f91a011d479bbd63f87a37808bfde379 previous none completion 61ea2251870cb8c223513c3362d60739
