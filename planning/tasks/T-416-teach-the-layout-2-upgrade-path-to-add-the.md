---
id: T-416-teach-the-layout-2-upgrade-path-to-add-the
title: Teach the layout-2 upgrade path to add the artifacts ignore rule
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-398-ignore-artifacts-on-committed-init
updated_at: "2026-09-17T20:24:47Z"
last_verification_id: "3669bbd9e7b531e427514fd2caefc6c8"
last_verification_result: fail
last_verified_at: "2026-09-15T21:29:15Z"
---

# T-416-teach-the-layout-2-upgrade-path-to-add-the Teach the layout-2 upgrade path to add the artifacts ignore rule

## Description

Make a valid committed-mode layout-1 to layout-2 upgrade establish the same
artifact-ignore invariant as fresh committed-mode init: after a successful
apply, files below the configured planning directory's `artifacts/` tree do not
appear as untracked Git output without a later init. The preview and apply must
describe and publish one consistent migration candidate, preserve unrelated and
already-equivalent user `.gitignore` bytes, and keep the existing durable
rollback/recovery boundary intact. Local-mode exclusion remains separate and
must not change.

This is the deferred migration half of T-398-ignore-artifacts-on-committed-init.
It owns upgrade-path behavior and regression evidence. Fresh committed
init/retrofit policy, ignore-rule redesign, spec or CI changes, release-gate
wiring, and deletion or rewriting of user-authored ignore rules are out of
scope.

## Acceptance

- In a temporary Git repository containing a valid layout-1 committed Taskrail
  state and no artifact ignore rule, a write-free upgrade preview reports the
  `.gitignore` candidate without changing repository bytes. A gated apply then
  publishes layout 2 and passes `taskrail validate`; a file created below the
  configured planning `artifacts/` directory is absent from
  `git status --porcelain --untracked-files=all`.
- When `.gitignore` contains unrelated user content, including content without
  a trailing newline, the successful upgrade preserves those bytes exactly and
  adds at most one marked artifact rule. An existing equivalent rule, existing
  Taskrail block, or deliberate user negation is preserved byte-for-byte and is
  not overridden or duplicated by the upgrade or a later idempotent init.
- A deterministic failure after the artifact-ignore candidate has been
  published but before migration completes never leaves an unfenced layout-2
  marker/state paired with the pre-upgrade ignore state. A completed rollback
  restores the original `.gitignore` bytes; a completed recovery that accepts
  the candidate leaves the new ignore rule together with the final layout-2
  state, and both outcomes preserve unrelated user files.
- Local-mode initialization and skill refresh continue to use Git's effective
  exclude rather than adding or changing a worktree `.gitignore`.

## Verification Notes

- For the upgrade-path contract, seed a real temporary Git repository with a
  valid layout-1 marker, state, and task set, omit the artifact rule, and run
  preview followed by the required apply gate. Compare a pre-preview tree
  snapshot with the preview result; then use the JSON write inventory, the
  published marker/state, `taskrail validate`, and Git status as the durable
  oracles for candidate parity and ignored artifact output.
- Exercise a focused `.gitignore` matrix covering unrelated bytes with and
  without a final newline, equivalent rules, the marked block, and a deliberate
  negation. Assert exact user-byte preservation, rule semantics, and
  idempotence through the existing ignore-policy boundary rather than only
  checking that a file exists.
- Use the existing deterministic durable-transaction failure/interruption seam
  after the candidate write set includes `.gitignore`. Assert exact rollback
  bytes and, separately, recovery acceptance bytes and validation; verify that
  no partial ignore change becomes visible outside the transaction boundary.
- Retain a focused local-mode regression for `.git/info/exclude` and run a
  reproducible sandbox CLI probe for preview, gated apply, artifact creation,
  validation, and status. Record the focused test and manual-evidence paths in
  the implementation verification record.

## Implementation Notes

- 2026-09-15T21:29:12Z: Pre-start sizing gate: unauthored placeholder follow-up - outcome, acceptance, and verification sections retain unedited TODO scaffold lines, so a verified result cannot be reached without unresolved scope and improvised criteria cannot support a pass. Needs human-approved task author body authoring or reviewed decomposition before execution.
- 2026-09-15T21:29:15Z: verification fail id 3669bbd9e7b531e427514fd2caefc6c8 previous none completion none
- 2026-09-17T20:24:47Z: Authoring approved: replace the placeholder body with a bounded outcome, acceptance, and verification plan; implementation and verification remain outstanding.
