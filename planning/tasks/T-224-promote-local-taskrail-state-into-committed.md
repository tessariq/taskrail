---
id: T-224-promote-local-taskrail-state-into-committed
title: Promote local Taskrail state into committed planning
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-166-publish-workflow-review-index-and-reports-with-cas
    - T-247-install-packaged-skills-safely-in-local-mode
updated_at: "2026-08-05T22:04:28Z"
---

# T-224-promote-local-taskrail-state-into-committed Promote local Taskrail state into committed planning

## Description

Provide a dry-run-first adoption boundary that converts one valid ignored local
Taskrail ledger into reviewable committed planning without losing task identity,
rewriting human bodies, publishing local artifacts, or exposing data before the
committed candidate validates.

## Acceptance

- `local promote` preview reports every source/destination/reference/exclusion
  change, linked-worktree consequence, collision, and committed validation result
  with zero writes.
- Apply rejects mixed state, destination/index conflicts, unsafe paths, source
  changes during the transaction, affected sibling installations, and ambiguous
  skill promotion. Historical origin drift alone is an advisory, not refusal.
- The transactional candidate switches config mode while preserving the logical
  path namespace plus task IDs/statuses/timestamps/bodies, excludes local
  artifacts/runtime data, and publishes specs/tasks/state/config plus durable
  NOTES/review/prompt files all-or-none under the durable transaction.
- After post-validation, apply removes only local semantic/prompt files whose
  exact bytes were published and then managed exclusions. Interruption or cleanup
  failure retains shared recovery so committed and local semantic stores are not
  simultaneously accepted.
- Success leaves committed planning visible for operator review/staging but makes
  no Git commit; `--with-skills` is the only skill-promotion consent and exact
  text/JSON candidate/result fields agree between preview and apply.

## Verification Notes

- Exercise empty/populated/custom-path ledgers, linked worktrees, every destination
  collision, source races, publication faults, exclusion-removal faults, rollback,
  and successful validation in temporary Git repositories.
- Compare exact semantic task/spec/state snapshots before and after promotion and
  prove artifacts plus unconsented skills remain local.

## Implementation Notes
