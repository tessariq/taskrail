---
id: T-165-maintain-bounded-workflow-adversarial-review
title: Validate workflow-adversarial evidence and memory
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-275-decode-workflow-adversarial-review-memory-strictly
updated_at: "2026-08-04T21:32:13Z"
---

# T-165-maintain-bounded-workflow-adversarial-review Validate workflow-adversarial evidence and memory

## Description

Provide strict, reusable validation for workflow-adversarial reports, prior memory,
and their bound repository snapshots. Index derivation, publication, and the
packaged workflow remain separate outcomes.

## Acceptance

- A1. Exact report and index objects enforce IDs, terminal typed observation
  evidence, resolvable references, a role-mandated v1 prompt source/template
  binding, separate outcome/freshness, canonical ordering, 1 MiB input cap, and
  at most three explained surface keys per run.
- A2. Canonical JSON and recorded-HEAD-tree product hash framing produce
  byte-identical digests
  across platforms; obsolete closure uses superseding evidence while resolved and
  not-reproducible require fresh executed attempts.
- A3. Snapshot validation binds clean attached HEAD, selected spec, product tree,
  prior index or absence, globally unique review ID, and exact before/after index
  digests; changed snapshots and invalid durable/transient paths refuse.
- A4. Finding transitions require human-created full task IDs for tracked, fresh
  reproduction for resolved, evidence/rationale for other closures, and
  checked finding visibility; clean surfaces require executed observable evidence.
- A5. Freshness assessments cover affected prior rows and validate changed-path
  evidence while making no semantic claim that a retained row is unaffected.

## Verification Notes

- A1-A5: strict positive and mutation fixtures cover fields, references, ordering,
  caps, every disposition, product framing, dirty/detached snapshots, digest/path
  drift, uniqueness, and rollover freshness.
- A4/A5: a two-run fixture proves clean evidence becomes stale by default and a
  finding cannot close without the required executed evidence.

## Implementation Notes
