---
id: T-403-name-non-portable-review-id-refusals
title: Name non-portable review ids in prompt path refusals
status: todo
priority: low
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-09-11T16:28:46Z"
---

# T-403-name-non-portable-review-id-refusals Name non-portable review ids in prompt path refusals

## Description

In `taskrail-workflow-adversarial-local` the agent chose review id
`WF-evaluation-20260910a`, which fails the portable review key
`^[a-z0-9]+(?:-[a-z0-9]+)*$`. `prompt render` refused with "transient prompt
path ... is outside its REVIEW_PATH proposal subtree"
(`internal/taskrail/prompt_transient.go`), and the agent misreported a conflict
between the reported artifacts directory and the renderer. The uppercase id is
the likely cause; confirm with a probe.

Outcome: a non-portable review id is refused with a message that names it.

## Acceptance

- A proposal path whose review-id segment is not portable yields a refusal
  naming the id and the portability rule, distinct from a wrong-prefix refusal.
- The machine error code stays as specified.

## Verification Notes

- Unit tests for both refusal variants.

## Implementation Notes
