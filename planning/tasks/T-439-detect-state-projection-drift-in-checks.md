---
id: T-439-detect-state-projection-drift-in-checks
title: Detect STATE.md projection drift in checks
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies: []
updated_at: "2026-10-31T11:39:14Z"
---

# T-439-detect-state-projection-drift-in-checks Detect STATE.md projection drift in checks

## Description

Design risk. A hand edit of a task's `status: todo` to `completed` passes
`taskrail validate`; a completed task with no `completion_id` looks like the
allowed legacy case, and the stale STATE.md counts are reported only by
`taskrail repair` as a body diff. No hook, script or CI job runs that drift
check, so hand edits that AGENTS.md forbids can land unnoticed. Changing CI or
hooks needs maintainer approval. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- One check fails when `taskrail repair` would change STATE.md (a validate signal or a dedicated command).
- The check runs in the CI planning lane and the opt-in pre-commit hook.
- The docs state the limit: the check detects projection drift, not who made a transition.

## Verification Notes

- Hand-edit a status in a temp repo and confirm the check fails; confirm a clean repo passes.

## Implementation Notes
