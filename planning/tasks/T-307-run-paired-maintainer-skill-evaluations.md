---
id: T-307-run-paired-maintainer-skill-evaluations
title: Run paired maintainer skill evaluations
status: completed
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-218-add-maintainer-skill-release-evaluations
updated_at: "2026-08-27T12:41:08Z"
completion_id: "cd0c753ddfc9e23c4b1d104f81a9d0df"
last_verification_id: "0e691558e67021a97a0fcb3487915870"
last_verification_result: pass
last_verified_at: "2026-08-27T12:41:08Z"
last_verified_completion_id: "cd0c753ddfc9e23c4b1d104f81a9d0df"
---

# T-307-run-paired-maintainer-skill-evaluations Run paired maintainer skill evaluations

## Description

Ship the provider-neutral maintainer runner and strict safe-summary report builder
for paired candidate-versus-v0.4.0 execution of the complete T-218 registry.

## Acceptance

- A1. A caller-owned adapter runs every registered case once in its declared mode
  against candidate and required baseline arms, retaining complete raw trees only
  beneath the status-reported transient artifacts root with normative digests.
- A2. The runner binds clean tested HEAD/product, candidate and fixed baseline skill
  trees, fixture inventory, selected executable digests, intended/observed
  adapter/model identity, deterministic grades, missing arms, and human comparisons.
- A3. The canonical schema-v1 report contains every case exactly once in required
  order and derives `pass|fail|incomplete` by exact precedence; absent credentials,
  missing/incomplete arms, inconclusive comparisons, and false-favorable aggregates
  cannot become pass. Base reports require null waiver.
- A4. Analysis and reruns occur only in an isolated workspace and may emit patch
  proposals, but cannot edit skills, mirrors, fixtures/assertions, tracked state,
  Git history, or choose/apply a winner.
- A5. The maintainer procedure produces an uncommitted safe report candidate with
  digest-only raw references and reproducible human-review questions; T-174 owns
  final release-snapshot execution/publication and T-249 owns waiver semantics.

## Verification Notes

- A1-A3: strict positive/mutation suites cover complete pairing, new skills,
  missing/incomplete arms, failing baselines, executable/snapshot/digest mismatch,
  ordering, outcome precedence, identity truthfulness, and raw tree framing.
- A4/A5: a sandbox candidate-vs-release run records deterministic and blind human
  comparison, emits but does not apply a patch proposal, and validates an
  uncommitted path-safe report candidate without changing protected repository bytes.

## Implementation Notes

- 2026-08-27T12:40:47Z: Added a strict provider-neutral paired evaluation runner and safe report renderer with raw-evidence containment.
- 2026-08-27T12:41:08Z: verification pass id 0e691558e67021a97a0fcb3487915870 previous none completion cd0c753ddfc9e23c4b1d104f81a9d0df
