---
id: T-352-ignore-transient-git-locks-in-test-snapshots
title: Ignore transient Git locks in test snapshots
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-308-publish-deterministic-loop-selection-and-dry-run
updated_at: "2026-08-22T15:07:50Z"
completion_id: "79e0f829d9b06d45b2964668904cb2b8"
last_verification_id: "ab5238b5b4b186a74e6b0001237312ca"
last_verification_result: pass
last_verified_at: "2026-08-22T15:07:50Z"
last_verified_completion_id: "79e0f829d9b06d45b2964668904cb2b8"
---

# T-352-ignore-transient-git-locks-in-test-snapshots Ignore transient Git locks in test snapshots

## Description

Keep repository side-effect assertions deterministic while Git creates and
removes internal maintenance locks asynchronously.

Follow-up derived from T-308-publish-deterministic-loop-selection-and-dry-run's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

## Acceptance

- Test snapshots exclude only `.git`-internal paths ending in `.lock`.
- Entries that disappear between traversal and reading do not fail a snapshot;
  all other filesystem errors still fail loudly.
- Durable repository and Git metadata remain represented in snapshots.
- The dry-run and full test suites pass on native macOS.

## Verification Notes

- Unit-test stable transient-lock exclusion, run focused dry-run tests, vet, the
  full suite, validation, task-body checks, and exact-head native macOS CI.

## Implementation Notes

- 2026-08-22T15:07:50Z: Excluded transient Git lockfiles and tolerated vanished traversal entries in repository test snapshots.
- 2026-08-22T15:07:50Z: verification pass id ab5238b5b4b186a74e6b0001237312ca previous none completion 79e0f829d9b06d45b2964668904cb2b8
