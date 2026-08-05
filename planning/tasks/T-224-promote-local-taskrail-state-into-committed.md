---
id: T-224-promote-local-taskrail-state-into-committed
title: Promote local Taskrail state into committed planning
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-156-protect-existing-semantic-writers-with-snapshot
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
- Apply rejects mixed state, destination/index conflicts, unsafe paths, branch or
  source drift, affected sibling local installations, and ambiguous skill
  promotion before publication.
- The transactional candidate switches config mode while preserving the logical
  path namespace plus task IDs/statuses/timestamps/bodies, excludes local
  artifacts/runtime locks, and publishes specs/tasks/state/config plus durable
  NOTES/review files all-or-none under the mutation lock.
- Managed exclusion entries are removed last and only after post-validation;
  failure retains recoverable local source and restores unchanged originals
  without hiding or overwriting external edits.
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
