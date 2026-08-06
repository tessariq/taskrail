---
id: T-180-make-semantic-publication-durably-transactional
title: Extend durable transactions for storage and Git index moves
status: todo
priority: high
spec_ref: specs/v0.6.0.md#atomic-git-aware-moves-and-recovery
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-08-04T23:06:23Z"
---

# T-180-make-semantic-publication-durably-transactional Extend durable transactions for storage and Git index moves

## Description

Extend the completed v0.5 durable transaction substrate with Git-index snapshots,
handle-bound directory creation, and committed/local live-to-archive move state.

## Acceptance

- The inherited engine acquires ownership before initial
  resolver/allocator/candidate reads and holds through validation, publication,
  post-validation, rollback, or retained recovery.
- Manifests extend inherited path snapshots with explicit original/candidate/current
  Git index states where committed mode stages a rename; local mode records
  unchanged index/status and managed-overlay identities instead.
- Publication/rollback use per-component compare-and-swap; clean outcomes
  remove only owned data while death/failure retains reconstructable blocking
  backups.
- APIs expose stable complete read-set/phase checks consumed by archive/restore but
  do not reimplement the inherited shared recovery command.
- The reusable protocol can include an absent configured human-owned sidecar in
  init/migration write sets, preserving existing bytes and rolling back only the
  transaction's newly created IDEAS file without interpreting its contents.

## Verification Notes

- Map criteria to pre-read races, Git/non-Git/linked/delegated locks,
  permissions/symlink attacks, external edits, absence, sync ordering, death,
  and reconstruction.
- Property-test conditional state transitions with portable processes.

## Implementation Notes
