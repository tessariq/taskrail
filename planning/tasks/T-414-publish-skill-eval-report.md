---
id: T-414-publish-skill-eval-report
title: Publish the skill evaluation report through resume
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-12T07:08:16Z"
---

# T-414-publish-skill-eval-report Publish the skill evaluation report through resume

## Description

`SkillEvalRunner.Resume` exists and already takes the staged record, the run
input, the overall `human_review` text, and the per-case reviews. What is
missing is the maintainer-facing driver: the manual harness only calls
`Execute` and writes `stage.json`, so a sealed stage plus its adopted worksheet
answers cannot be turned into the committed release report that the v0.5.0 gate
requires at
`<planning-dir>/reviews/skill-evals/v0.5.0/<session-id>/report.json`.

Add that driver, so the human comparison boundary is a real stopping point: a
maintainer stages once, answers the worksheet over however many sittings it
takes, and resumes without re-invoking a single arm. Resume already rejects
schema, seal, snapshot, registry, executable, skill, fixture, raw-tree, and
worksheet/review drift, so the driver's own job is narrow: read the stage and
the answers, reconstruct the same run input and bindings the staging run used,
call `Resume`, and write the rendered report.

## Acceptance

- The driver reads a sealed `stage.json` and an answers file holding the overall
  `human_review` plus each case's `comparison` and `human_review`, reconstructs
  the staging run's input and bindings, and writes the rendered report to
  `<planning-dir>/reviews/skill-evals/v0.5.0/<session-id>/report.json`.
- No adapter arm is invoked. A stage whose bindings no longer match the current
  snapshot, registry, executable, skill, fixture, or raw trees is refused by
  `Resume` and the driver writes nothing.
- An answers file that omits a case, names an unregistered case, or supplies a
  comparison the case's completeness does not permit fails before any write.
- The written report is canonical two-space-indented UTF-8 JSON with one final
  LF, contains no producer-local path, and passes the committed-summary
  validator that rejects absolute paths, `~/`, and `raw/` substrings.
- Re-running the driver over the same stage and answers writes byte-identical
  output.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes
