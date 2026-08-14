---
id: T-322-provide-handle-bound-durable-filesystem-primitives
title: Provide handle-bound durable filesystem primitives
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-222-initialize-and-discover-ignored-local-taskrail
    - T-317-bind-delegated-grants-to-the-owner-s-declared-task
updated_at: "2026-08-14T13:12:51Z"
---

# T-322-provide-handle-bound-durable-filesystem-primitives Provide handle-bound durable filesystem primitives

## Description

Provide the native filesystem substrate required by durable transactions: bind
managed paths to stable identities without following links, publish and replace
relative to retained directory handles, and make file and directory durability
barriers explicit on Linux, macOS, and Windows. This task owns primitives and
their failure model, not transaction phases or recovery policy.

## Acceptance

- A1. Supported platforms open and classify existing ancestors and leaf entries
  without following symlinks or reparse points, retain a stable identity, and
  refuse aliases, hard-link ambiguity, special entries, and identity substitution.
- A2. Handle-relative create, no-replace publication, replacement, removal, and
  directory creation cannot escape the bound root or silently retarget after an
  ancestor rename or replacement.
- A3. File-content, metadata, and parent-directory durability barriers expose a
  precise success or classified unsupported/failure result; success never means
  only that buffered bytes reached an API boundary.
- A4. Restart-time identity checks compare persisted portable identity evidence
  with freshly bound handles and fail closed when the platform or filesystem
  cannot prove the required relationship.
- A5. Linux, macOS, and Windows use native implementations behind one narrow
  standard-library-style interface; `golang.org/x/sys` is the only approved new
  runtime dependency, and unsupported filesystems refuse rather than weakening
  guarantees.

## Verification Notes

- A1-A2: race ancestor/leaf substitution, links, aliases, and rename boundaries
  against every operation and assert no out-of-root or wrong-identity mutation.
- A3: inject each file and directory sync failure and prove success is reported
  only after the complete ordered barrier sequence.
- A4: close and reopen fixtures, including replaced ancestors and leaves, and
  compare the persisted evidence with the rebound identity.
- A5: run native tests on the current host and cross-compile all supported target
  implementations; retain platform-specific runtime coverage in CI-facing tests.

## Implementation Notes
