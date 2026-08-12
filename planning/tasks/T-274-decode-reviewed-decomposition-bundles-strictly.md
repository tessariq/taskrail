---
id: T-274-decode-reviewed-decomposition-bundles-strictly
title: Decode reviewed decomposition bundles strictly
status: blocked
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-240-implement-the-normative-review-schema-decoders
updated_at: "2026-08-12T12:35:05Z"
---

# T-274-decode-reviewed-decomposition-bundles-strictly Decode reviewed decomposition bundles strictly

## Description

Strictly decode reviewed decomposition draft, trace, adversarial review, and
manifest bundles so only a complete digest-bound task set can cross the review
publication and import boundaries.

## Acceptance

- A1. ImportDraft v2 and trace documents enforce exact schemas, portable unique
  keys, selected-spec anchors, source/range rules, and bidirectional requirement-to-
  task coverage without accepting v1-only meaning.
- A2. Review passes enforce role-mandated prompt bindings, pass order and cap,
  exact draft/trace snapshots, unique findings, and final-byte review semantics.
- A3. The manifest binds the post-spec review, spec, draft, trace, ordered reviews,
  and complete finding dispositions by exact identity and digest.
- A4. A complete valid bundle decodes without byte normalization; missing files,
  extra files, broken references, stale digests, cycles, or deferrable high/medium
  findings are rejected deterministically.

## Verification Notes

- A1: decode aligned v2 draft/trace goldens and mutate keys, anchors, ranges,
  version-specific fields, and both directions of trace coverage.
- A2: exercise one- and two-pass sessions plus wrong role, non-consecutive pass,
  stale snapshot, duplicate finding, and post-final-change cases.
- A3: mutate every subject/review digest and disposition relationship in a complete
  manifest and observe focused rejection.
- A4: publish-boundary fixtures cover exact-byte acceptance and each bundle
  membership/reference failure class.

## Implementation Notes

- 2026-08-12T12:34:54Z: Third and final correctness review found unresolved current-scope Markdown parsing defects: Setext H1 headings bypass the reviewed-body top-level-heading prohibition, and unseparated trailing # characters are incorrectly stripped as ATX closers. Fixing after review three would leave changed final bytes unreviewed. The standalone taskrail:check also cannot validate the mandated runner wrapper without bypassing it.
- 2026-08-12T12:35:05Z: verification fail
