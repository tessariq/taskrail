---
id: T-224-promote-local-taskrail-state-into-committed
title: Promote local semantic state into committed planning
status: completed
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-305-publish-workflow-review-reports-and-memory
    - T-247-install-packaged-skills-safely-in-local-mode
updated_at: "2026-08-28T09:37:50Z"
completion_id: "1f4f15240ff1ceb36e0c21e1b6e8b0c6"
last_verification_id: "a4f2263867c0c31691f1a2c2dac42562"
last_verification_result: pass
last_verified_at: "2026-08-28T09:37:50Z"
last_verified_completion_id: "1f4f15240ff1ceb36e0c21e1b6e8b0c6"
---

# T-224-promote-local-taskrail-state-into-committed Promote local semantic state into committed planning

## Description

Provide the dry-run-first transactional boundary that converts one valid ignored
local semantic store into reviewable committed planning. Preserve installed skill
bytes and exclusions as an explicit pending-skill state; T-315 owns all consented
skill visibility changes, including the combined path.

## Acceptance

- `local promote` preview reports every semantic source/destination/reference/
  exclusion change, linked-worktree consequence, collision, and committed
  validation result with zero writes. Managed semantic entries use logical paths;
  config/artifact/runtime entries use repository-root physical paths, with local
  artifact/runtime values confined to the transient result.
- Apply rejects mixed state, destination/index conflicts, unsafe paths, source
  changes during the transaction, affected sibling installations, and any skill
  state that cannot be preserved exactly. Historical origin drift alone is an
  advisory, not refusal.
- The transactional candidate switches config mode while preserving the logical
  path namespace plus task IDs/statuses/timestamps/bodies, excludes local
  artifacts/runtime data, and publishes specs/tasks/state/config plus durable
  NOTES/review/prompt files all-or-none under the durable transaction.
- After post-validation, apply removes only local semantic/prompt files whose
  exact bytes were published and then managed exclusions. Interruption or cleanup
  failure retains shared recovery so committed and local semantic stores are not
  simultaneously accepted.
- Success leaves committed planning visible for operator review/staging but makes
  no Git commit. Installed local skills at normal assistant paths and all their
  managed exclusions remain byte-identical, leaving the sole valid committed-mode
  pending-skill state for T-315. Repositories without installed managed skills
  leave no pending state.
- Result `writes`, `preserved`, and `excluded` kinds obey the closed v0.5 semantic
  contract: writes/preserved never classify artifact/runtime as published, and
  excluded never classifies published semantic/config content. Skill actions are
  `preserve_local` or `absent`; this task never reports `promote`.
- Preview actions describe candidates only; apply plus top-level `applied:true`
  is the sole claim that semantic visibility changed. Exact
  text/JSON candidate fields and validation agree between preview and apply.

## Verification Notes

- Exercise empty/populated/custom-path ledgers, linked worktrees, every destination
  collision, source races, publication faults, exclusion-removal faults, rollback,
  and successful validation in temporary Git repositories.
- Compare exact semantic task/spec/state snapshots before and after promotion and
  prove artifacts/runtime and installed skill bytes/exclusions remain local while
  committed semantic files become visible without a Git commit.
- Exercise absent, valid installed, and ambiguous skill states to prove semantic
  promotion either preserves an exact T-315-compatible pending state or refuses
  atomically; consented combined/deferred visibility is tested by T-315.

## Implementation Notes

- 2026-08-28T09:37:44Z: Implemented recoverable local promotion with durable semantic publication, local artifact/runtime preservation, custom-path support, and staged-config protection; verified by focused and full Go suites plus sandbox evidence.
- 2026-08-28T09:37:50Z: verification pass id a4f2263867c0c31691f1a2c2dac42562 previous none completion 1f4f15240ff1ceb36e0c21e1b6e8b0c6
