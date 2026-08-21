---
id: T-284-protect-importdraft-v1-publication-transactionally
title: Protect ImportDraft v1 publication transactionally
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-234-protect-repository-and-planning-writers
updated_at: "2026-08-21T07:53:10Z"
completion_id: "2714f191f837641aed26190eba742ff0"
---

# T-284-protect-importdraft-v1-publication-transactionally Protect ImportDraft v1 publication transactionally

## Description

Protect legacy `ImportDraft` v1 apply with one normal transaction spanning every
created spec, task, and state projection. ImportDraft parsing/scaffold semantics
stay compatible; this task owns only validated all-or-none publication.

## Acceptance

- Apply snapshots the source draft, config, existing ledger, destination namespace,
  and state before constructing and validating the complete import candidate under
  the repository lock.
- Every proposed spec/task path is collision-checked and the exact candidate ledger
  validates before the first publication; unrelated files remain byte-identical.
- A successful result names only bytes actually published and a source change or
  destination race refuses with the applicable common machine error.
- Any handled failure after publication removes newly created unchanged paths and
  restores replaced unchanged bytes, or retains exact conflict/recovery evidence;
  no partial-success result is emitted.

## Verification Notes

- Import multi-task/spec v1 drafts into temporary repositories and compare exact
  source, task, spec, state, and sentinel bytes with the reported write set.
- Inject source/collision races and faults after each publication and rollback
  boundary, asserting all-or-none output and schema-1 error snapshots.
- Confirm preview remains write-free and v1 scaffold/body interpretation is
  unchanged by the transaction integration.

## Implementation Notes

- 2026-08-21T07:52:57Z: Published legacy import apply as a normal transaction with race-safe validation and rollback.
- 2026-08-21T07:53:10Z: verification pass
