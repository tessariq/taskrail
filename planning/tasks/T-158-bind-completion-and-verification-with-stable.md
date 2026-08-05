---
id: T-158-bind-completion-and-verification-with-stable
title: Bind completion and verification with stable identities
status: todo
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-04T21:32:13Z"
---

# T-158-bind-completion-and-verification-with-stable Bind completion and verification with stable identities

## Description

Define one canonical workflow contract and make completion/verification
identities mechanically distinguish fresh evidence from stale or audit evidence.
Persist the exact identity contract consistently across task, state, JSON,
report, notes, and artifact names.

## Acceptance

- Complete creates a random lower-case 32-hex completion ID, clears stale passing
  bindings, and the canonical docs are cited rather than redefined by prompts,
  skills, AGENTS, and tests.
- Every verify creates a preflight-absent random lower-case 32-hex verification ID
  and artifact directory `<timestamp>-<verification-id>/report.json`, plus exact
  task note and state summary; pass binds observed completion and fail binds none.
- First passing verification of a legacy completed task atomically adopts a
  completion ID before binding; failure never adopts one, and fault leaves all
  surfaces unchanged.
- Pass before any non-completed status remains advisory, writes evidence without
  status/validation changes, warns on human stderr, emits the exact JSON warning
  object, and fail emits no advisory.
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
- Use fault injection and frozen clocks to prove atomic legacy adoption and
  ID/set-based freshness rather than timestamp freshness.
- Confirm verification-created follow-ups remain implicitly held without changing
  any existing task's `loop_policy` or `loop_reason`.

## Implementation Notes
