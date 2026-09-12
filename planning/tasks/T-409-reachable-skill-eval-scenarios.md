---
id: T-409-reachable-skill-eval-scenarios
title: Seed skill-eval fixtures so each case prompt is reachable
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-09-11T16:28:49Z"
---

# T-409-reachable-skill-eval-scenarios Seed skill-eval fixtures so each case prompt is reachable

## Description

Many registered cases ask for flows their fixture cannot reach, so arms only
exercise refusals: recovery and repair seed no drift; autonomous-verify seeds
no invalid evidence; retrofit runs on an already-initialized repository and
always gets `destination_exists`; loop cases set no loop policy, so the dry run
is always `action: none`; sdd-handoff seeds no OpenSpec or Spec Kit artifacts;
decompose seeds no published post-spec review; spec seeds no target spec; gap
and decompose have zero coverable areas (T-399). The
`taskrail-retrofit-committed` baseline invented "reviewed" notes to fill the gap.

Outcome: every case's prompt claims only what its setup establishes.

## Acceptance

- For each registered case, the positive or recovery condition its prompt names
  exists after setup, or the prompt is narrowed to what does.
- Registry validation mechanically checks what it can (setup establishes the
  named condition).

## Verification Notes

- Registry tests; a model-backed replay of changed cases before the next
  release evaluation. Coordinate with T-399.

## Implementation Notes
