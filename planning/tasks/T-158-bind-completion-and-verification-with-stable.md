---
id: T-158-bind-completion-and-verification-with-stable
title: Bind completion and verification with stable identities
status: todo
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-229-canonicalize-v0-5-lifecycle-and-task-identities
    - T-233-protect-lifecycle-and-task-writers-transactionally
updated_at: "2026-08-04T21:32:13Z"
---

# T-158-bind-completion-and-verification-with-stable Bind completion and verification with stable identities

## Description

Implement completion and chained verification identities from the canonical
contract, persisting exact current evidence across task, state, JSON, report,
notes, and artifact names. Lifecycle prose ownership and pass-before-complete
warning are separate tasks.

## Acceptance

- Complete creates a random lower-case 32-hex completion ID and clears all prior
  task-level verification fields while preserving repository-level history.
- Every verify creates a preflight-absent random lower-case 32-hex verification ID,
  records the exact prior task verification ID or null, and writes the artifact,
  note, task tuple, state tuple, command result, and report consistently.
- First passing verification of a legacy completed task atomically adopts a
  completion ID before binding; failure never adopts one, and fault leaves all
  surfaces unchanged.
- Pass before completion remains unbound evidence without status change; T-241
  owns its advisory representation.
- Task, state, command JSON, canonical summary, task note, complete artifact
  path/report, and report fields agree after fresh/stale pass, completed audit
  fail, repeated complete, and recovery-only verify.
- Follow-ups created by verification carry no unattended authorization and
  therefore omit `loop_policy` and `loop_reason` and remain on implicit hold until
  a direct operator action allows them.

## Verification Notes

- Map each criterion to setup/action/public observation/evidence across focused
  transition tests, exact golden outputs, filesystem snapshots, and a manual
  lifecycle matrix.
- Use fault injection and frozen clocks to prove atomic legacy adoption, direct
  predecessor chaining, and ID/set-based freshness rather than timestamps.
- Confirm verification-created follow-ups remain implicitly held without changing
  any existing task's `loop_policy` or `loop_reason`.

## Implementation Notes
