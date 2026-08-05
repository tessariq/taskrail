---
id: T-163-validate-and-apply-importdraft-v2-transactionally
title: Validate and apply ImportDraft v2 transactionally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-161-apply-reviewed-task-bodies-with-compare-and-swap
    - T-162-productize-digest-bound-post-spec-review-lenses
updated_at: "2026-08-04T21:32:13Z"
---

# T-163-validate-and-apply-importdraft-v2-transactionally Validate and apply ImportDraft v2 transactionally

## Description

Implement strict ImportDraft v2 validation and its all-or-none writer so complete
reviewed bodies, traceability, review inputs, and approval manifests are bound to
exact imported bytes. Preserve v1 behavior explicitly.

## Acceptance

- V2 exact schemas require complete valid bodies, one review session,
  bidirectional trace keys, real anchors, unique keys, acyclic true dependencies,
  immutable ordered passes, and strict unknown/null/duplicate rejection.
- Apply verifies layout 2, lock participation, spec-review manifest, spec, draft,
  trace, every review, and decomposition-manifest path/digest/session identity
  before any write.
- Task, spec, and state candidates all stage and validate as one repository
  before atomic publication; every snapshot is rechecked and
  failure/interruption rolls back without overwriting concurrent bytes.
- Exact non-empty reviewed body bytes reach tasks; final session files publish
  no-follow/no-alias/no-clobber with tasks, while abandoned proposals are
  excluded.
- Imported tasks contain no separate loop-policy output or authorization. Their
  `loop_policy` and `loop_reason` fields are absent and therefore implicitly held,
  and v2 rejects drafts that attempt to set them.
- V1 retains documented partial/scaffold behavior and cannot consume v2 fields
  or silently discard a draft presented as v2.

## Verification Notes

- Map criteria to strict decoder fixtures, exact body snapshots, digest/session
  races, trace/cycle faults, path publication races, and fault injection at every
  task/spec/state boundary.
- Compare v1 compatibility and v2 all-or-none observations after interruption and
  concurrent edits.

## Implementation Notes
