---
id: T-408-reword-eval-transcript-question
title: Reword the skill-eval transcript review question for real agent runs
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-09-11T16:28:49Z"
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
