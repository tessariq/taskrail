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
    - T-215-add-the-generic-review-artifact-publisher
updated_at: "2026-08-04T21:32:13Z"
---

# T-163-validate-and-apply-importdraft-v2-transactionally Validate and apply ImportDraft v2 transactionally

## Description

Implement strict ImportDraft v2 validation and its all-or-none writer so complete
reviewed bodies, traceability, review inputs, and approval manifests are bound to
exact imported bytes. Preserve v1 behavior explicitly.

## Acceptance

- V2 exact field schemas require complete valid bodies, one review session,
  bidirectional trace keys, real anchors, unique keys, acyclic true dependencies,
  immutable ordered passes, last-review binding to exact final draft/trace bytes,
  and strict unknown/null/duplicate rejection.
- Apply verifies layout 2, lock participation, spec-review manifest, spec, draft,
  trace, every review, and decomposition-manifest path/digest/session identity
  before any write.
- Every v2 apply uses the exact published draft/manifest command surface, targets
  tasks with empty spec sections, and requires a final decomposition review;
  unreviewed and spec-writing imports remain v1-only.
- Task, spec, and state candidates all stage and validate as one repository
  before atomic publication; every snapshot is rechecked and
  failure/interruption rolls back without overwriting concurrent bytes.
- Exact non-empty reviewed body bytes reach tasks; previously published final
  session files are immutable validated inputs to apply, while abandoned
  proposals are excluded and never copied or rewritten by import.
- Imported tasks contain no separate loop-policy output or authorization. Their
  `loop_policy` and `loop_reason` fields are absent and therefore implicitly held,
  and v2 rejects drafts that attempt to set them.
- V1 retains documented partial/scaffold behavior for v0.5 and cannot consume v2
  fields or silently discard a draft presented as v2; v0.6 explicitly upgrades
  publication durability without changing body interpretation.
- V2 remains source-agnostic: an explicit future `import ideas --to tasks` may
  produce its reviewed task draft, but source prose never bypasses complete body,
  real-anchor, trace, review, digest, or implicit-hold requirements.

## Verification Notes

- Map criteria to strict decoder fixtures, exact body snapshots, digest/session
  races, trace/cycle faults, path publication races, and fault injection at every
  task/spec/state boundary.
- Compare v1 compatibility and v2 all-or-none observations after interruption and
  concurrent edits.

## Implementation Notes
