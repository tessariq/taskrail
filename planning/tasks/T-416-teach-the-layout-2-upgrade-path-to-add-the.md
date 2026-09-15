---
id: T-416-teach-the-layout-2-upgrade-path-to-add-the
title: Teach the layout-2 upgrade path to add the artifacts ignore rule
status: blocked
priority: medium
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-398-ignore-artifacts-on-committed-init
updated_at: "2026-09-15T21:29:15Z"
last_verification_id: "3669bbd9e7b531e427514fd2caefc6c8"
last_verification_result: fail
last_verified_at: "2026-09-15T21:29:15Z"
---

# T-416-teach-the-layout-2-upgrade-path-to-add-the Teach the layout-2 upgrade path to add the artifacts ignore rule

## Description

Deferred independently meaningful outcome: The layout-1 to layout-2 migration path does not manage the worktree .gitignore, so a migrated repository keeps planning/artifacts/ visible in Git status until a later plain init adds the marked block (verified in T-398 review as finding T398-3 and deferred as out of that task's fresh-init scope). Make the upgrade path manage the same marked artifacts ignore rule fresh committed-mode init writes, preserving user rules byte-for-byte.

This task owns integrated delivery of the deferred outcome and its invariant after T-398-ignore-artifacts-on-committed-init's verification.

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes

- 2026-09-15T21:29:12Z: Pre-start sizing gate: unauthored placeholder follow-up - outcome, acceptance, and verification sections retain unedited TODO scaffold lines, so a verified result cannot be reached without unresolved scope and improvised criteria cannot support a pass. Needs human-approved task author body authoring or reviewed decomposition before execution.
- 2026-09-15T21:29:15Z: verification fail id 3669bbd9e7b531e427514fd2caefc6c8 previous none completion none
