---
id: T-234-protect-repository-and-planning-writers
title: Protect repository and planning writers transactionally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-06T13:46:30Z"
---

# T-234-protect-repository-and-planning-writers Protect repository and planning writers transactionally

## Description

Retrofit init, retrofit, repair, spec, and legacy import/planning writers to the
shared transaction substrate while preserving each command's declared durability
class and read-only previews.

## Acceptance

- A1. Every repository/planning writer declares and enforces one exact consumed and
  published set under the correct Git-common or non-Git root-local lock.
- A2. Legacy ImportDraft v1 handled failures roll back all candidate task/spec/state
  writes and use schema-1 error details instead of a partial success result.
- A3. Preview/read paths take no lock that mutates state, create no transaction
  files, and report only a stable rechecked snapshot.

## Verification Notes

- A1: a command matrix records expected lock and write sets, then uses sentinels to
  detect undeclared writes.
- A2: import failure injection after each candidate publication observes original
  bytes plus structured snapshots.
- A3: before/after filesystem and Git-index digests prove read-only purity.

## Implementation Notes
