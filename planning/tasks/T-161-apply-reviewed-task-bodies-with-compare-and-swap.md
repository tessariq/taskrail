---
id: T-161-apply-reviewed-task-bodies-with-compare-and-swap
title: Apply reviewed task bodies with compare-and-swap safety
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-159-add-a-versioned-workflow-prompt-catalog
updated_at: "2026-08-04T21:32:13Z"
---

# T-161-apply-reviewed-task-bodies-with-compare-and-swap Apply reviewed task bodies with compare-and-swap safety

## Description

Add the read-only task-authoring prompt and guarded task author writer, then make
scaffolded, imported, follow-up, and decomposition-producing guidance share one
outcome-focused body contract.

## Acceptance

- Task author requires the expected exact digest, layout 2 and writer lock,
  rechecks todo status/bytes, and atomically changes only Description, Acceptance,
  and Verification Notes.
- Frontmatter, H1, identity, lifecycle fields, and Implementation Notes remain
  byte-equivalent, including `loop_policy` and `loop_reason`; conflict, validation
  failure, interruption, or non-todo prompt rendering writes nothing.
- Bodies require one meaningful outcome, observable acceptance,
  criterion-to-evidence setup/action/observation/layer mapping, public oracles,
  integrated criteria where needed, and relevant negative boundaries without
  unnecessary internals.
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
- Manually author a scaffolded task and confirm criterion-to-evidence quality plus
  exact machine-managed-byte preservation.

## Implementation Notes
