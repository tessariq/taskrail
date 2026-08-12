---
id: T-275-decode-workflow-adversarial-review-memory-strictly
title: Decode workflow adversarial review memory strictly
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-240-implement-the-normative-review-schema-decoders
updated_at: "2026-08-12T18:35:10Z"
---

# T-275-decode-workflow-adversarial-review-memory-strictly Decode workflow adversarial review memory strictly

## Description

Strictly decode workflow-adversarial reports and canonical review memory so serial
testing can update bounded, referentially sound evidence without accepting stale,
cyclic, ambiguous, or lossy history.

## Acceptance

- A1. Workflow reports enforce exact prompt/subject snapshots, bounded surface
  scope, portable identities, ordered unique references, evidence combinations,
  finding transitions, and freshness assessments.
- A2. Workflow indexes enforce canonical encoding, monotonic finding allocation,
  bounded rows/bytes, exact unresolved-finding shapes, and immutable historical
  identity fields.
- A3. Every report/index reference resolves in its declared scope; observation
  cycles, proposal-path evidence, invalid closure evidence, and unexplained
  freshness retention are rejected.
- A4. Applying a valid report to absent or strict prior memory derives one exact
  candidate index digest while preserving unexplained rows and refusing overflow
  rather than dropping unresolved findings.

## Verification Notes

- A1: decode valid report variants and mutate snapshots, identities, ordering,
  evidence kinds, transitions, scope cap, and freshness decisions.
- A2: round-trip canonical index goldens and reject noncanonical ordering/encoding,
  reused counters, invalid historical changes, and configured caps.
- A3: exercise dangling, duplicate, cyclic, transient-path, and insufficient
  closure/freshness evidence graphs.
- A4: compare first-run and prior-memory candidate bytes/digests, then force each
  cap and observe refusal without history loss.

## Implementation Notes

- 2026-08-12T18:34:42Z: Added strict workflow-adversarial report and canonical INDEX.json decoding plus mechanical candidate-index derivation with cap refusal, referential integrity, and history-immutability rules.
- 2026-08-12T18:35:10Z: verification pass
