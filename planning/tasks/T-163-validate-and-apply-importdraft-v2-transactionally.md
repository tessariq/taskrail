---
id: T-163-validate-and-apply-importdraft-v2-transactionally
title: Validate and apply ImportDraft v2 transactionally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies:
    - T-252-validate-reviewed-decomposition-bundles
updated_at: "2026-08-04T21:32:13Z"
---

# T-163-validate-and-apply-importdraft-v2-transactionally Validate and apply ImportDraft v2 transactionally

## Description

Apply one already validated and published ImportDraft v2 bundle through a durable
all-or-none transaction while preserving exact reviewed body bytes. T-252 owns
strict bundle/schema/relation validation.

## Acceptance

- Apply verifies layout 2, lock participation, spec-review manifest, spec, draft,
  trace, every review, and decomposition-manifest path/digest/session identity
  before any write.
- Every v2 apply uses the exact published draft/manifest command surface, targets
  tasks with empty spec sections, and requires a final decomposition review;
  unreviewed and spec-writing imports remain v1-only.
- Task and projected-state candidates stage and validate against the selected
  read-only spec before atomic publication; every snapshot is rechecked and
  failure/interruption rolls back without overwriting concurrent bytes.
- Exact non-empty reviewed body bytes reach tasks; previously published final
  session files are immutable validated inputs to apply, while abandoned
  proposals are excluded and never copied or rewritten by import.
- Imported tasks contain no separate loop-policy output or authorization. Their
  `loop_policy` and `loop_reason` fields are absent and therefore implicitly held,
  and v2 rejects drafts that attempt to set them.
- V1 transaction integration remains owned by T-234; this outcome neither changes
  its scaffold/body meaning nor reimplements its handled rollback. Only v2 receives
  durable crash recovery in v0.5.
- V2 remains source-agnostic: an explicit future `import ideas --to tasks` may
  produce its reviewed task draft, but source prose never bypasses complete body,
  real-anchor, trace, review, digest, or implicit-hold requirements.

## Verification Notes

- Map criteria to strict decoder fixtures, exact body snapshots, digest/session
  races, trace/cycle faults, path publication races, and fault injection at every
  task/state boundary.
- Compare v1 compatibility and v2 all-or-none observations after interruption and
  concurrent edits.

## Implementation Notes
