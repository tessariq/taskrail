---
id: T-198-verify-v0-6-identity-archival-and-recovery
title: Verify v0.6 identity archival and recovery contracts
status: todo
priority: high
spec_ref: specs/v0.6.0.md#behavioral-and-release-acceptance
dependencies:
    - T-187-create-opaque-tasks-through-commands-and
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-191-add-stable-task-inspection-and-filtered-inventory
    - T-194-add-explicit-archive-and-restore-commands
    - T-195-report-unified-ledger-storage-without-semantic
    - T-196-integrate-stable-references-with-rename-prompts
    - T-197-extend-loop-postflight-for-storage-and
updated_at: "2026-08-04T23:06:23Z"
---

# T-198-verify-v0-6-identity-archival-and-recovery Verify v0.6 identity archival and recovery contracts

## Description

Complete cross-surface behavioral, documentation, native move, and release
checks proving identity, cancellation, archival, migration, and recovery deliver
one workflow.

## Acceptance

- Registry/drift checks cover selectors, schemas, storage, warnings, recovery,
  debt, allocator entry points, explicit write sets, task-local loop fields, and
  stable-read contracts without duplicating feature tests.
- One end-to-end sandbox upgrades layout, creates generated/opaque tasks,
  resolves a dependency, completes/verifies or cancels/remediates, archives,
  shows/lists/reports invariantly, restores, and proves restore-before-edit.
- Native Windows/macOS/Linux evidence is required for handle-bound Git
  moves/inverse restore; native Windows additionally covers opaque filenames,
  while broader scenarios use reproducible packaged sandboxes.
- Docs consistently describe stable refs, roots, restore-before-edit,
  cancellation, path remediation, recovery, downgrade limits, and task-local
  `loop_policy`/`loop_reason` with absent fields as implicit hold. Documentation
  and behavior tests reject stale `AUTONOMY.tsv` and separate policy
  storage/publication assumptions as well as stale lifecycle/live-only paths,
  while preserving provider independence.
- Tag release gates run exact CI/checklist acceptance before any publisher and
  tests prove a failed gate skips GoReleaser/all publication jobs.

## Verification Notes

- Map criteria to cross-contract tests, integrated workflow report,
  platform/toolchain/binary-digest evidence, docs assertions, provider scans,
  stale-policy scans, and release-job dependency tests.
- Mutation-test selector/schema/lifecycle docs so unrelated prose cannot satisfy
  them.

## Implementation Notes
