---
id: T-343-make-local-inspection-tests-portable
title: Make local inspection tests portable across native filesystems
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-288-inspect-local-planning-storage-read-only
updated_at: "2026-08-21T10:03:27Z"
---

# T-343-make-local-inspection-tests-portable Make local inspection tests portable across native filesystems

## Description

Repair the T-288 local-inspection fixtures so unsupported native filesystem
durability is skipped consistently and platform-specific aliases of the same Git
common directory do not make the linked-worktree assertion fail.

Follow-up derived from T-288-inspect-local-planning-storage-read-only's verification or discovery.

## Acceptance

- Every T-288 fixture that requires successful `init --local` first applies the
  existing directory-durability capability gate.
- Linked-worktree scope assertions compare canonical filesystem identities rather
  than lexical paths while retaining exact worktree and common-directory checks.
- Linux behavior remains covered, and the previously failing native Windows and
  macOS CI cases pass without weakening product behavior.

## Verification Notes

- Run focused local-inspection tests, repository validation, vet, and the full Go
  suite; confirm exact-head native Windows and macOS CI.

## Implementation Notes
