---
id: T-283-protect-repair-and-spec-writers-transactionally
title: Protect repair and spec writers transactionally
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-234-protect-repository-and-planning-writers
updated_at: "2026-08-21T07:46:58Z"
completion_id: "d71b2ce4b1d59d4503e336244d48c143"
---

# T-283-protect-repair-and-spec-writers-transactionally Protect repair and spec writers transactionally

## Description

Route `repair --apply`, `spec add`, and `spec activate` through the shared normal
transaction substrate. This slice owns structural state/spec publication while
preserving repair/spec read-only forms as stable snapshots.

## Acceptance

- Each apply command locks the discovered repository, snapshots every consumed
  config/spec/task/state path and destination collision, validates its complete
  candidate, and publishes only the command's declared spec/state files.
- `repair --apply` remains a state-only mechanical reprojection; `spec activate`
  changes only active-spec state fields; `spec add` uses its no-clobber spec
  publication point and never overwrites an existing destination.
- Repair preview and all read-only spec forms create no lock or transaction files
  and emit only after rechecking the consumed snapshot.
- Handled failures roll back unchanged publications and common machine errors
  expose exact snapshots/recovery without modifying task files.

## Verification Notes

- Use command-specific before/after snapshots and task/spec sentinels to prove the
  exact write set and mechanical invariants.
- Inject destination races and failures at publication, post-validation, and
  rollback boundaries; assert no-clobber and external-edit preservation.
- Compare repository and lock-root digests around repair/spec read-only forms.

## Implementation Notes

- 2026-08-21T07:46:58Z: Published repair and spec writers through normal transactions with no-clobber spec creation and stable read snapshots.
- 2026-08-21T07:46:58Z: verification pass
