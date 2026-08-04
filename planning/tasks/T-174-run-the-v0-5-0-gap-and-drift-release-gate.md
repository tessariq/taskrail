---
id: T-174-run-the-v0-5-0-gap-and-drift-release-gate
title: Run the v0.5.0 gap and drift release gate
status: todo
priority: high
spec_ref: specs/v0.5.0.md#goals
dependencies:
    - T-173-check-cross-surface-workflow-contract-integrity
updated_at: "2026-08-04T21:32:13Z"
---

# T-174-run-the-v0-5-0-gap-and-drift-release-gate Run the v0.5.0 gap and drift release gate

## Description

Perform the final v0.5.0 semantic gap, drift, exclusion, and release-readiness
review from a fresh implementation/spec/task snapshot after every implementation
and remediation task is complete. Do not tag or claim current until it passes.

## Acceptance

- Every goal, feature, caution, recommendation, and exclusion is classified
  against implementation, tests, packaged skills, docs, and release notes.
- Coverage is 100 percent, every structural signal has a disposition, and
  independent semantic/adversarial review leaves no blocker.
- Full formatting, vet, tests, race, cross-build, parity, bodies, freshness,
  validation, release build/snapshot, checklist, clean tree, CI, Planning,
  CodeQL, migration, and native Linux/macOS/Windows packaged evidence passes.
- Every current-version blocker becomes a standalone remediation task and direct
  gate dependency, explicitly not a follow-up-of the gate; the gate stops and
  later restarts the review on fresh bytes. Cancelled dependencies never satisfy
  it.
- Changelog/README become final only after all other criteria; final verify occurs
  only with no open release remediation, and tagging remains a maintainer action.

## Verification Notes

- Map each criterion to the semantic matrix, command logs, remote URLs,
  native/manual reports, Git/task dependency observations, and final fresh
  verification.
- In a sandbox create a standalone blocker, add only the gate-to-remediation
  dependency, prove no cycle and gate ineligibility, complete it, then restart
  review.

## Implementation Notes
