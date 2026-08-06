---
id: T-193-move-task-storage-with-handle-bound-git
title: Move task storage with handle-bound transactions
status: todo
priority: high
spec_ref: specs/v0.6.0.md#atomic-git-aware-moves-and-recovery
dependencies:
    - T-181-detect-durable-physical-task-path-references
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-185-upgrade-repositories-transactionally-to-layout-3
updated_at: "2026-08-04T23:06:23Z"
---

# T-193-move-task-storage-with-handle-bound-git Move task storage with handle-bound transactions

## Description

Implement the cross-platform no-follow move engine that binds ancestry by handle,
stages an exact rename in committed mode, performs an ignored-overlay rename in
local mode, and recovers interrupted states.

## Acceptance

- Require clean non-bare root-equal Git state. Committed mode permits the exact
  inverse staged archive rename; local mode requires complete ignored managed
  storage and leaves index/visible status unchanged.
- Bind root/source/destination handles and create missing parents
  handle-relatively; source attributes/filters/index semantics fail closed as
  specified.
- Native rename plus mode-specific exact staging or no-Git local publication
  preserves bytes/mode and expected logical storage without path-resolving git mv.
- Final full read-set/path/eligibility/attribute check authorizes success;
  changes roll back conditionally and command-created empty directories remove
  in reverse order only.
- Signals forward/wait for Git; failure reports both paths, Git stage/status,
  bounded byte comparison, original plus rollback errors, and exact recovery
  state without reset/deletion/overwrite.

## Verification Notes

- Map criteria to Git cleanliness/inverse, handle/parent/attribute/stage races,
  cleanup boundaries, diagnostics, signals/wait, every location state, and
  recovery.
- Persist native Windows/macOS/Linux staged-rename and inverse-restore reports.

## Implementation Notes
