---
id: T-417-keep-local-mode-usable-after-a-rolled-back-local
title: Keep local mode usable after a rolled-back local promote leaves empty committed dirs
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-400-local-mode-committed-skill-exclusion
updated_at: "2026-09-17T23:38:06Z"
completion_id: "eccb45dad184bc3c66a60a07716a161c"
last_verification_id: "f3796dd67a7fb96bff695be1fa498b86"
last_verification_result: pass
last_verified_at: "2026-09-17T23:38:06Z"
last_verified_completion_id: "eccb45dad184bc3c66a60a07716a161c"
---

# T-417-keep-local-mode-usable-after-a-rolled-back-local Keep local mode usable after a rolled-back local promote leaves empty committed dirs

## Description

After an explicit local `promote --apply` aborts and its durable transaction
rolls back after creating committed destination roots, local storage remains
discoverable and usable. Empty committed `specs/` and `planning/` roots left
solely by that failed attempt may be pruned or treated as harmless scaffolding,
but committed Taskrail content remains a mixed-state error. Rollback cleanup
must not delete user-owned bytes; successful promotion, local artifact/runtime
exclusions, and T-400's tracked-skill ownership behavior remain unchanged.

This is the deferred rollback-recovery outcome from
T-400-local-mode-committed-skill-exclusion. It does not broaden local-mode
discovery to accept non-empty committed state, erase user content, redesign the
promotion protocol, or change T-174's dependency graph.

## Acceptance

- In a valid temporary local-mode Git repository with semantic state to promote,
  a deterministic post-publication promotion failure is driven to a completed
  rollback. A subsequent `taskrail local status --json` succeeds, reports local
  storage with no mixed-state violation, and leaves the local semantic bytes,
  local artifacts/runtime bytes, and managed exclusions usable. Any committed
  `specs/` or `planning/` directories left by the failed attempt are absent or
  empty, not a source of a false refusal.
- A repository with actual committed Taskrail content, such as a regular file
  under `specs/` or `planning/`, still refuses local discovery/status with the
  existing mixed-state error. The change must not turn non-empty committed
  state into an accepted local-mode repository.
- Rollback and empty-directory handling preserve every pre-existing user-owned
  file, symlink, and non-empty directory byte-for-byte; cleanup removes only
  transaction-created empty structure when cleanup is the chosen behavior.
- A successful local promotion still publishes a valid committed candidate,
  preserves the documented local artifact/runtime bytes and exclusions, and
  passes committed-mode validation. The T-400 distinction between tracked and
  untracked packaged skills remains unchanged.

## Verification Notes

- Set up a real temporary Git repository with explicit local initialization,
  at least one valid local task/spec/state payload, and a clean promotion
  destination. Use the existing deterministic durable-transaction failure seam
  after committed destination directories have been created, run
  `local promote --apply`, and drive rollback to completion. Use the CLI
  `local status --json` result, a fresh discovery, exact tree snapshots, and
  exclusion contents as the public and durable oracles; do not treat only an
  exit code or directory existence as proof.
- Seed a separate local-mode fixture with a non-empty committed `specs/` or
  `planning/` path and prove `local status` refuses with the mixed-state error
  without changing repository bytes. Include a pre-existing user file and an
  external edit in the rollback fixture to prove cleanup does not remove or
  overwrite user content.
- Re-run the existing successful semantic and `--with-skills` promotion cases,
  including tracked and untracked packaged copies, and assert the committed
  candidate validates after apply.
- Perform a reproducible sandbox CLI probe covering local initialization,
  promotion failure/rollback, post-rollback `local status`, and the real
  mixed-state refusal. Record the focused test and manual-evidence paths in the
  implementation verification record.

## Implementation Notes

- 2026-09-15T21:33:02Z: Pre-start sizing gate: unauthored placeholder follow-up - outcome, acceptance, and verification sections retain unedited TODO scaffold lines, so a verified result cannot be reached without unresolved scope and improvised criteria cannot support a pass. Needs human-approved task author body authoring or reviewed decomposition before execution.
- 2026-09-15T21:33:08Z: verification fail id 19a1f31a97e3d90cb1f68b7bbee1a497 previous none completion none
- 2026-09-17T20:24:54Z: Authoring approved: replace the placeholder body with a bounded outcome, acceptance, and verification plan; implementation and verification remain outstanding.
- 2026-09-17T23:38:01Z: Local-mode discovery treats committed specs/, planning/, and .taskrail/prompts/ roots that hold no file, symlink, or special entry as harmless scaffolding instead of mixed committed/local state, so a completed rollback of local promote --apply (which restores every local byte but cannot remove the transaction-created empty destination roots) leaves local status, local path, fresh discovery, and promotion usable. Non-empty committed roots still refuse with the unchanged mixed-state error. Focused tests: internal/taskrail/local_promote_rollback_test.go (TestLocalPromoteRollbackKeepsLocalModeUsable drives a real post-publication failure to completed rollback through the new deterministic validate-closure seam and proves byte-identical local/user/exclusion restoration plus usable local status; TestLocalDiscoveryRefusesNonEmptyCommittedRootsAfterRollbackShape pins the non-empty refusal, including a nested committed file). Manual CLI evidence: sandboxed probe under the ignored planning/artifacts tree (init --local, task new, replicated rollback residue, post-rollback local status --json mode=local promotion_ready=true violations=[], local path, promote preview, validate, and the real mixed-state refusal with unchanged bytes). One General+Go review round; the single validated low finding (early-exit plus contextualized walk error in the emptiness predicate) was fixed with a deliberate regression proving the strengthened refusal test fails, and the fresh disposition-verification context returned RESOLVED with no new issues.
- 2026-09-17T23:38:06Z: verification pass id f3796dd67a7fb96bff695be1fa498b86 previous none completion eccb45dad184bc3c66a60a07716a161c
