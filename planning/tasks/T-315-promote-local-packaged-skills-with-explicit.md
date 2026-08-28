---
id: T-315-promote-local-packaged-skills-with-explicit
title: Promote local packaged skills with explicit consent
status: completed
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-224-promote-local-taskrail-state-into-committed
    - T-302-refresh-local-packaged-skills-safely
updated_at: "2026-08-28T10:29:15Z"
completion_id: "3edb824007019db84daba9e54a6ef1f2"
last_verification_id: "d7758ae75fa8f64a0e71a9f6d7d722e3"
last_verification_result: pass
last_verified_at: "2026-08-28T10:29:15Z"
last_verified_completion_id: "3edb824007019db84daba9e54a6ef1f2"
---

# T-315-promote-local-packaged-skills-with-explicit Promote local packaged skills with explicit consent

## Description

Add explicit `--with-skills` consent to local promotion. Validate unchanged
Taskrail-owned installed skill bytes and remove only their exact managed
exclusions, both atomically with semantic promotion and later from the sole
committed pending-skill state.

## Acceptance

- During local semantic promotion, `--with-skills` selects the combined candidate:
  every managed installed skill destination is reclassified, its unchanged bytes
  and ownership/version parity are validated, and only its exact managed
  exclusion is removed on the same durable transaction and recovery boundary as
  T-224's semantic publication.
- Without `--with-skills`, combined promotion preserves every skill byte and
  exclusion and leaves T-224's explicit pending-skill state. Consent never copies,
  rewrites, installs, refreshes, or silently adopts skill content.
- In that pending state only, `local promote --with-skills` preview/apply is valid
  with `source_mode:"committed"`, `target_mode:"committed"`, empty semantic
  `writes`/`preserved`/`excluded` arrays, and only exact skill/exclusion candidates.
  Apply removes exclusions without changing skill bytes or creating a Git commit.
- Omitted consent in pending state, no pending managed exclusion, repeated apply,
  mixed/ambiguous/changed skill bytes, ownership or parity failure, linked-
  worktree conflict, and destination/exclusion race are `unsupported` or the
  narrower registered refusal and write nothing.
- Preview and apply expose exact ordered `skills` actions
  (`promote|preserve_local|absent`) and `removed_exclusions`. Preview `promote`
  means only that visibility would change; only `applied:true` claims removal.
- Combined and deferred apply are all-or-none durable transactions. Interruption,
  semantic publication failure, exclusion-removal failure, or post-validation
  failure restores prior visibility or retains shared recovery; it never leaves
  partially visible skill roots or a falsely cleared pending state.

## Verification Notes

- Temporary repositories exercise combined preview/apply, semantic-only deferral,
  deferred preview/apply, no-installed-skill `absent`, omitted consent, repeated
  calls, and exact text/JSON candidate equivalence.
- Byte/mode/index/status snapshots prove skill files are never rewritten or
  committed and that only consented exact exclusions change; linked-worktree,
  marker/parity, alias, case/Unicode, and destination-race fixtures fail closed.
- Fault injection at semantic publication, each exclusion removal,
  post-validation, and cleanup proves atomic rollback or retained recovery for
  both combined and deferred paths.

## Implementation Notes

- 2026-08-28T10:29:02Z: Added consented combined and deferred packaged-skill visibility promotion with atomic exclusion removal and parity checks.
- 2026-08-28T10:29:15Z: verification pass id d7758ae75fa8f64a0e71a9f6d7d722e3 previous none completion 3edb824007019db84daba9e54a6ef1f2
