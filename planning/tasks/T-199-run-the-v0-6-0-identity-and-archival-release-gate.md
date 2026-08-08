---
id: T-199-run-the-v0-6-0-identity-and-archival-release-gate
title: Run the v0.6.0 identity and archival release gate
status: todo
priority: high
spec_ref: specs/v0.6.0.md#goals
dependencies:
    - T-198-verify-v0-6-identity-archival-and-recovery
updated_at: "2026-08-04T23:06:23Z"
---

# T-199-run-the-v0-6-0-identity-and-archival-release-gate Run the v0.6.0 identity and archival release gate

## Description

Perform the final v0.6.0 semantic gap, drift, exclusion, and release-readiness
review from fresh candidate bytes after all implementation/remediation.

## Acceptance

- Every goal, feature, caution, recommendation, and exclusion is classified
  against implementation, tests, packages, docs, and release notes.
- Coverage is complete, every structural/semantic/adversarial signal is
  disposed, and no identity reuse, data-loss, false-clean, portability,
  migration, or downgrade blocker remains.
- Final drift review proves `loop_policy` and `loop_reason` remain task-local and
  preserved, all creation/import/follow-up paths use implicit hold, and no
  `AUTONOMY.tsv` or separate policy storage/publication assumption remains.
- Final drift review also proves transitions require complete digest-bound
  dispositions and reviewed task drafts, release-check remains read-only and
  non-authoritative for tagging, embedded skill inspection never materializes
  files, and agent mode changes representation rather than command semantics.
- Full CI/race/native packaged/migration/map/crash recovery/Git
  move/parity/body/release/checklist/remote/clean-tree evidence passes and binds
  candidate commit SHA, binary digest, platform/toolchain, commands, and
  results.
- Any spec/source/task/remediation change invalidates affected evidence and
  restarts review; blockers become standalone direct gate dependencies, never
  follow-up-of gate.
- README/changelog finalizes only after all criteria, final verification
  requires no open remediation, and tagging remains maintainer-owned.

## Verification Notes

- Map criteria to digest-bound semantic matrix, command/native/manual reports,
  remote URLs, dependency graph, stale-policy scans, and fresh final
  verification.
- Sandbox one synthetic blocker to prove non-cyclic insertion, evidence
  invalidation, and review restart before the real gate.

## Implementation Notes
