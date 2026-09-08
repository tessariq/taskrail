---
id: T-388-split-release-gate-deterministic-and-semantic
title: Split the release gate into a deterministic gate and a sampled semantic review
status: todo
priority: high
spec_ref: specs/v0.6.0.md#read-only-spec-release-check
dependencies: []
updated_at: "2026-09-08T07:27:34Z"
---

# T-388-split-release-gate-deterministic-and-semantic Split the release gate into a deterministic gate and a sampled semantic review

## Description

The v0.5.0 release gate (T-174) carries eleven acceptance bullets, about 465
words against a repository average of 130, and eleven remediation
dependencies. It bundles mechanical checks that CI already proves (formatting,
vet, tests, cross-builds, parity, coverage, validation) with sampled human
judgement (semantic gap review, decomposition sizing, skill-evaluation
transcript reading). One pass cannot verify both, so every attempt found a
different blocker and restarted the whole gate three times.

`taskrail spec release-check` is the mechanical half and is explicitly "not the
release procedure". This task gives the v0.6.0 release its other half as a
separate, bounded, sampled review instead of one unverifiable audit.

The independently meaningful outcome is that v0.6.0 ships through two tasks
with distinct verification surfaces: a deterministic gate that
`spec release-check` plus CI can green without a human, and a semantic review
with an explicit sample size and stop condition that a human can finish in one
sitting.

## Acceptance

- `docs/workflow/releasing.md` defines the two-stage gate: stage one is
  mechanical (`spec release-check`, CI, cross-builds, parity, task-body hygiene,
  binary freshness) and stage two is semantic review with a stated sample
  (areas, tasks, skill-evaluation cases) and a stop condition; stage two never
  re-runs stage one and a stage-two finding files a remediation task rather than
  reopening stage one.
- T-199 is re-scoped to stage one only, with acceptance that a mechanical run can
  satisfy; a new task owns stage two for v0.6.0 and T-199 depends on it.
- Neither task's acceptance exceeds the repository's sizing guidance in
  `AGENTS.md` (one independently meaningful outcome with a bounded verification
  surface); the combined bullet count is materially below T-174's.
- The skill-evaluation run that stage two requires is bounded in advance
  (number of cases and arms, and which skills need a baseline arm) so a human
  can complete transcript review in one session.

## Verification Notes

- Diff T-199 before and after; count acceptance bullets and words for T-199,
  the new stage-two task, and T-174 and record them.
- Dry-run `spec release-check v0.6.0 --json` once the command exists and show
  that every stage-one criterion maps to a check code or a CI job.
- Walk `docs/workflow/releasing.md` in a sandbox clone and confirm no step
  requires a human to re-verify a stage-one outcome.

## Implementation Notes
