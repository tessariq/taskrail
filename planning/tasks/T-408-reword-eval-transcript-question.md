---
id: T-408-reword-eval-transcript-question
title: Reword the skill-eval transcript review question for real agent runs
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-11-01T19:26:56Z"
completion_id: "a79344551c20cb3e57e37d0c81c067f8"
last_verification_id: "cf6f45b512410a6a7f14650d69cbdfd7"
last_verification_result: pass
last_verified_at: "2026-11-01T19:26:56Z"
last_verified_completion_id: "a79344551c20cb3e57e37d0c81c067f8"
---

# T-408-reword-eval-transcript-question Reword the skill-eval transcript review question for real agent runs

## Description

Every registered skill-eval case's second human review question asks whether
"the stubbed agent transcript" followed the skill. Release evaluations run real
agents; several agents in session v050-final-20260910t103851z read the wording
literally and reported that no stubbed transcript existed. The question text is
part of the fixture digests, so the change must land before a re-run.

Outcome: the question describes the transcript the evaluation actually has.

## Acceptance

- No registered case refers to a stubbed transcript.
- Registry validation passes.

## Verification Notes

- Registry test asserting the wording.

## Implementation Notes

- 2026-11-01T19:26:56Z: Corrected transcript wording in all registered evaluation cases; added registry-wide wording regression coverage. Independent review found no issues; deterministic checks passed. No model-backed evaluations run.
- 2026-11-01T19:26:56Z: verification pass id cf6f45b512410a6a7f14650d69cbdfd7 previous none completion a79344551c20cb3e57e37d0c81c067f8
