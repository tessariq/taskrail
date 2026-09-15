---
id: T-401-align-proposal-directory-precondition
title: Align review skills with the existing proposal directory precondition
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies: []
updated_at: "2026-09-15T16:05:44Z"
completion_id: "9a5aa69c3254b56b43e94b05af4595f2"
last_verification_id: "ee0534271c00d5d8715697dfa0aa082b"
last_verification_result: pass
last_verified_at: "2026-09-15T16:05:44Z"
last_verified_completion_id: "9a5aa69c3254b56b43e94b05af4595f2"
---

# T-401-align-proposal-directory-precondition Align review skills with the existing proposal directory precondition

## Description

`prompt render` refuses a missing proposal directory ("transient prompt
proposal directory ... is missing", `internal/taskrail/prompt_transient.go`),
but `taskrail-spec-review` tells agents to choose "one absent proposal
directory" and `taskrail-workflow-adversarial` "an absent effectively ignored
`<proposal>`"; `taskrail-task-review` does not say. Every review arm in the
v0.5.0 skill evaluation improvised `mkdir -p` or marker files to get past it.

Outcome: the review skills and the renderer state one directory precondition.

## Acceptance

- The three review skills describe exactly the proposal-directory precondition
  the renderer enforces, or the renderer accepts what the skills describe.
- Committed skill copies are regenerated and `task check:skills` passes.

## Verification Notes

- Contract test tying the documented precondition to renderer behavior.

## Implementation Notes

- 2026-09-15T16:05:40Z: Aligned the three review skills with the renderer's enforced precondition: create the effectively ignored proposal directory before prompt render (rendering refuses a missing proposal directory), while proposal files stay absent until staged; added a contract test tying the documented precondition to AuthorizeTransientPromptPaths behavior; regenerated committed skill copies (check:skills passes).
- 2026-09-15T16:05:44Z: verification pass id ee0534271c00d5d8715697dfa0aa082b previous none completion 9a5aa69c3254b56b43e94b05af4595f2
