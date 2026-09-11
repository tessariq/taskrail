---
id: T-401-align-proposal-directory-precondition
title: Align review skills with the existing proposal directory precondition
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies: []
updated_at: "2026-09-11T16:28:45Z"
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
