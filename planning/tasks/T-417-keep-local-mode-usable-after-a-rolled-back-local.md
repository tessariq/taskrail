---
id: T-417-keep-local-mode-usable-after-a-rolled-back-local
title: Keep local mode usable after a rolled-back local promote leaves empty committed dirs
status: blocked
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-400-local-mode-committed-skill-exclusion
updated_at: "2026-09-15T21:33:08Z"
last_verification_id: "19a1f31a97e3d90cb1f68b7bbee1a497"
last_verification_result: fail
last_verified_at: "2026-09-15T21:33:08Z"
---

# T-417-keep-local-mode-usable-after-a-rolled-back-local Keep local mode usable after a rolled-back local promote leaves empty committed dirs

## Description

Deferred independently meaningful outcome: Deferred from T-400 review: a rolled-back local promote --apply can leave empty committed specs/ and planning/ directories behind (durabletx rollback restores file bytes but does not prune directories it created), after which local-mode discovery refuses every command with repository_invalid 'mixed committed/local Taskrail state' until the operator removes the empty directories by hand. Pre-existing machinery; the deterministic trigger found in review was eliminated by T-400's containment fix, so this is race-reachable hardening: prune empty committed destination directories after rollback, or tolerate empty specs//planning/ in the local-mode mixed-state check the way committed mode tolerates empty init scaffolds, with a regression test forcing a post-publication promotion failure and asserting local status still works afterwards.

This task owns integrated delivery of the deferred outcome and its invariant after T-400-local-mode-committed-skill-exclusion's verification.

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes

- 2026-09-15T21:33:02Z: Pre-start sizing gate: unauthored placeholder follow-up - outcome, acceptance, and verification sections retain unedited TODO scaffold lines, so a verified result cannot be reached without unresolved scope and improvised criteria cannot support a pass. Needs human-approved task author body authoring or reviewed decomposition before execution.
- 2026-09-15T21:33:08Z: verification fail id 19a1f31a97e3d90cb1f68b7bbee1a497 previous none completion none
