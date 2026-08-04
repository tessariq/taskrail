---
id: T-180-make-semantic-publication-durably-transactional
title: Make semantic publication durably transactional
status: todo
priority: high
spec_ref: specs/v0.6.0.md#atomic-git-aware-moves-and-recovery
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-08-04T23:06:23Z"
---

# T-180-make-semantic-publication-durably-transactional Make semantic publication durably transactional

## Description

Extend the completed v0.5 lock with reusable write-ahead backup, durable phase,
compare-and-swap publication/rollback, and blocking recovery-record primitives.

## Acceptance

- The engine acquires ownership before initial
  resolver/allocator/candidate reads and holds through validation, publication,
  post-validation, rollback, or retained recovery.
- Lock selection uses Git common directory only for root-equal worktrees and
  root-local otherwise; linked/delegated ownership stays coherent.
- Private no-follow recovery roots and files enforce owner-only permissions;
  manifests record absence, original bytes/modes/index, candidate digests, and
  durable synced phase before publication.
- Publication/rollback use per-component compare-and-swap; clean outcomes
  remove only owned data while death/failure retains reconstructable blocking
  backups.
- APIs expose stable complete read-set/phase checks but do not claim command
  adoption.

## Verification Notes

- Map criteria to pre-read races, Git/non-Git/linked/delegated locks,
  permissions/symlink attacks, external edits, absence, sync ordering, death,
  and reconstruction.
- Property-test conditional state transitions with portable processes.

## Implementation Notes
