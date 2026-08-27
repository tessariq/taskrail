---
id: T-291-prove-inherited-command-parity-in-local-storage
title: Prove inherited command parity in local storage
status: completed
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-245-cover-the-complete-implicit-local-bootstrap-matrix
    - T-288-inspect-local-planning-storage-read-only
    - T-289-route-lifecycle-and-task-writers-through-local
    - T-290-route-structural-planning-writers-through-local
updated_at: "2026-08-27T08:54:41Z"
completion_id: "9b80af7bfda60decfc66083881facd13"
last_verification_id: "223e9dcb9e85fb4f617bd80fa2cfa371"
last_verification_result: pass
last_verified_at: "2026-08-27T08:54:41Z"
last_verified_completion_id: "9b80af7bfda60decfc66083881facd13"
---

# T-291-prove-inherited-command-parity-in-local-storage Prove inherited command parity in local storage

## Description

Provide the release-facing parity gate for every inherited command required to run
against local planning storage. This task owns exhaustive cross-mode integration
evidence and regressions, not new command semantics.

## Acceptance

- A maintained inventory maps every inherited v0.5 public command and form to
  committed/local support, read/write classification, bootstrap eligibility, and
  its owning focused test; no inherited surface is silently omitted.
- Every storage-neutral inherited reader and every lifecycle/task/structural writer
  produces equivalent logical results from equivalent committed/local semantic
  bytes, including exact schema-1 envelopes and human/JSON exit classification.
- Local runs touch only permitted fixed-overlay or explicitly managed operational
  paths, remain ignored/untracked/unstaged, and leave ordinary Git status clean;
  decoy committed files prove no direct-path bypass.
- Preview/read-only commands never bootstrap or write, eligible writers use the one
  implicit bootstrap implementation, and ineligible/unsupported forms refuse with
  no mixed state.
- The gate covers descendant cwd, custom logical directories, linked worktrees,
  origin drift, collisions, transaction faults, and rollback/recovery boundaries.

## Verification Notes

- Run one table-driven command inventory across paired committed/local fixtures,
  recording exact output, exit, filesystem, index, and status observations.
- Fail the test when a registered inherited command lacks a local case or when a
  local case accesses decoy logical paths outside the active context.
- Persist manual smoke evidence for one complete local workflow from init through
  selection, mutation, completion/verification, repair/spec/import, and inspection.

## Implementation Notes

- 2026-08-27T08:54:16Z: Added an enforced form-level local parity inventory with bootstrap and evidence-owner guards.
- 2026-08-27T08:54:41Z: verification pass id 223e9dcb9e85fb4f617bd80fa2cfa371 previous none completion 9b80af7bfda60decfc66083881facd13
