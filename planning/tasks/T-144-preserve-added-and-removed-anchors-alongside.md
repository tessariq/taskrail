---
id: T-144-preserve-added-and-removed-anchors-alongside
title: Preserve added and removed anchors alongside rename candidates
status: todo
priority: high
spec_ref: specs/v0.4.0.md#spec-version-diff
dependencies:
    - T-113-spec-diff
updated_at: "2026-07-29T13:04:20Z"
---

# T-144-preserve-added-and-removed-anchors-alongside Preserve added and removed anchors alongside rename candidates

## Description

`spec diff` computes definitive added/removed anchor sets, then removes any pair
matched by the best-effort rename heuristic from those lists. Because a rename is
only a candidate and never a fact, suppressing the mechanical delta hides the very
decomposition/orphan worklist the command promises.

## Acceptance

- `added` contains every anchor present only in the target spec and `removed`
  contains every anchor present only in the source spec, regardless of candidate
  pairing.
- `renamed` remains a deterministic, clearly labeled supplemental candidate list.
- Human and JSON output preserve document ordering and non-nil arrays.
- Tests assert that both sides of a candidate pair remain in the definitive lists
  and that read-only behavior is unchanged.

## Verification Notes

- T-140 sandbox evidence paired `spec-coverage-report` with
  `spec-coverage-summary` and omitted both from `removed`/`added`.

## Implementation Notes

- Keep `anchorSetDelta` authoritative; candidate detection should not consume its
  outputs.
