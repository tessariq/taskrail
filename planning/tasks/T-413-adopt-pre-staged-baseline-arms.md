---
id: T-413-adopt-pre-staged-baseline-arms
title: Adopt pre-staged baseline arms in skill evaluations
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-12T07:08:16Z"
---

# T-413-adopt-pre-staged-baseline-arms Adopt pre-staged baseline arms in skill evaluations

## Description

Every skill-evaluation baseline arm is pinned to fixed v0.4.0 inputs: the exact
baseline commit `62be3b13a67cbba51a4900b7ef6e192a645eb12d`, the baseline binary
built from it, that commit's skill subtree, and the case registry's
`fixtures_sha256`. A candidate-side product fix changes none of those, yet
`SkillEvalRunner.Execute` re-invokes all 11 baseline-required arms on every
staging run because it loops the registry unconditionally. That is roughly a
quarter of the provider time in each release-gate re-run, spent reproducing
byte-identical evidence.

Let a maintainer adopt specific baseline arms from raw trees already placed
under the new session's baseline raw root, instead of re-invoking the adapter
for them. Adoption must be verified, never trusted: the grade is re-derived
from the adopted `facts.json` through the case's own assertion-to-action
predicates, and `raw_sha256` is recomputed over the adopted tree, exactly as a
freshly run arm is. Adoption is opt-in and explicitly enumerated, so a stale
raw tree left behind by an earlier session can never be silently consumed.

The report schema does not change and no spec sentence requires a baseline arm
to execute inside the report's own session, so this needs no spec amendment;
adoption is disclosed as human evaluation evidence in the report's
`human_review` summary, alongside discarded retries.

## Acceptance

- `SkillEvalRunInput` carries an explicitly enumerated set of baseline-required
  case IDs eligible for adoption. Cases outside it always invoke the adapter.
- For an adopted case, `Execute` invokes no adapter arm, re-derives the
  deterministic grade from the adopted raw tree's recorded facts through the
  case's assertion-to-action predicates, and recomputes `raw_sha256` over that
  tree. A baseline run record produced by adoption is indistinguishable in the
  report from one produced by execution.
- Adoption refuses loudly rather than degrading: a listed case ID that is not
  registered, not `baseline_required`, or whose baseline raw root is missing or
  holds an empty tree fails the run. Facts that are missing, extra, fabricated,
  or receipt-mismatched fail exactly as for an executed arm.
- Candidate arms are never adoptable.
- The report's strict schema, digest preimages, and outcome precedence are
  unchanged, and `pass` remains reachable for a run whose baseline arms were
  adopted.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes
