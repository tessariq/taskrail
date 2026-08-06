---
id: T-161-apply-reviewed-task-bodies-with-compare-and-swap
title: Apply reviewed task bodies with compare-and-swap safety
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-251-ship-the-outcome-focused-task-authoring-prompt
updated_at: "2026-08-04T21:32:13Z"
---

# T-161-apply-reviewed-task-bodies-with-compare-and-swap Apply reviewed task bodies with compare-and-swap safety

## Description

Implement only the guarded `task author` writer and make existing task-producing
surfaces consume the reviewed body contract delivered by T-251.

## Acceptance

- Task author requires the expected exact digest, layout 2 and writer lock,
  rechecks todo status/bytes, and atomically changes only Description, Acceptance,
  and Verification Notes.
- Proposals contain exactly the three non-empty ordered level-2 sections with no
  frontmatter, H1, Implementation Notes, or other level-2 heading; successful
  text/JSON preserves proposal section bytes and reports the exact before/after
  digests, canonical unified-diff string, validation, and applied state.
- Frontmatter, H1, identity, lifecycle fields, and Implementation Notes remain
  byte-equivalent, including `loop_policy` and `loop_reason`; conflict, validation
  failure, interruption, or non-todo prompt rendering writes nothing.
- Task-new scaffolding and every task-producing built-in, prompt, and skill cite
  or emit that contract, including legacy v1 import/follow-up paths and reviewed
  v2 decomposition.
- Body text and body proposals cannot authorize unattended execution; only the
  dedicated direct loop-policy commands may change the paired frontmatter fields.
- Dry-run and JSON report a deterministic selected-task-only diff; malformed
  headings, frontmatter/H1, ignored artifact paths, and forbidden changes are
  rejected.

## Verification Notes

- Map criteria to digest/status races, forbidden changes, duplicate headings,
  rollback, loop-policy escalation attempts, successful byte preservation, and
  golden normal/follow-up/v1/v2 task fixtures.
- Manually apply a reviewed proposal and confirm exact machine-managed-byte preservation.

## Implementation Notes
