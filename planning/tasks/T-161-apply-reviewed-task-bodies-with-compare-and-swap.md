---
id: T-161-apply-reviewed-task-bodies-with-compare-and-swap
title: Apply reviewed task bodies with compare-and-swap safety
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-251-ship-the-outcome-focused-task-authoring-prompt
updated_at: "2026-08-25T20:18:01Z"
completion_id: "3a3499ed7e4fb7a90779e2a58f81520e"
last_verification_id: "b34f1d38e3b4f3d9f7680ce88d11a265"
last_verification_result: pass
last_verified_at: "2026-08-25T20:18:01Z"
last_verified_completion_id: "3a3499ed7e4fb7a90779e2a58f81520e"
---

# T-161-apply-reviewed-task-bodies-with-compare-and-swap Apply reviewed task bodies with compare-and-swap safety

## Description

Implement only the guarded `task author` writer that applies one reviewed body
proposal to an existing todo task. Producer alignment remains T-303 and T-304.

## Acceptance

- A1. `task author` requires the expected exact digest, layout 2 and writer lock,
  rechecks todo status/bytes, and atomically changes only Description, Acceptance,
  and Verification Notes.
- A2. Proposals contain exactly the three non-empty ordered level-2 sections with no
  frontmatter, H1, Implementation Notes, or other level-2 heading; successful
  text/JSON preserves proposal section bytes and reports the exact before/after
  digests, canonical unified-diff string, validation, and applied state.
- A3. Frontmatter, H1, identity, lifecycle fields, and Implementation Notes remain
  byte-equivalent, including `loop_policy` and `loop_reason`; conflict, validation
  failure, interruption, or non-todo target writes nothing.
- A4. Body text and body proposals cannot authorize unattended execution; only the
  dedicated direct loop-policy commands may change the paired frontmatter fields.
- A5. Dry-run and JSON report a deterministic selected-task-only diff; malformed
  headings, frontmatter/H1, ignored artifact paths, and forbidden changes are
  rejected.

## Verification Notes

- A1-A5: digest/status races, malformed proposals, forbidden-field and loop-policy
  escalation attempts, rollback faults, and a successful dry-run/apply pair prove
  exact diff/digest and managed-byte preservation.
- CLI sandbox evidence applies one reviewed proposal and observes only the three
  authorized body sections change.

## Implementation Notes

- 2026-08-25T20:17:45Z: Implemented digest-bound task authoring with transactional body-only updates.
- 2026-08-25T20:18:01Z: verification pass id b34f1d38e3b4f3d9f7680ce88d11a265 previous none completion 3a3499ed7e4fb7a90779e2a58f81520e
