---
id: T-322-provide-handle-bound-durable-filesystem-primitives
title: Provide handle-bound durable filesystem primitives
status: blocked
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-222-initialize-and-discover-ignored-local-taskrail
    - T-317-bind-delegated-grants-to-the-owner-s-declared-task
updated_at: "2026-08-14T13:35:45Z"
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

- A TDD candidate proved retained-parent ancestry, no-follow opening, native
  barriers, and restart evidence, but the bounded filesystem/security review
  found the strong A1-A2 leaf contract cannot be implemented with the approved
  native surfaces. Linux `renameat2`/`unlinkat` and macOS
  `renameatx_np`/`unlinkat` resolve a source name, not a retained source handle;
  verify-then-mutate therefore has an unavoidable substitution window. Neither
  platform provides an atomic expected-file-identity compare for rename/remove,
  and a concurrent actor can add a hard link between every link-count check and
  mutation.
- The same review found generic restart evidence cannot prove non-reuse across
  every supported filesystem: Linux mount IDs are not restart-stable and Windows
  volume/file IDs may be reused after deletion. Failing closed would leave no
  conforming Linux/macOS implementation for required operations, contrary to A5;
  weakening the adversarial contract was explicitly disallowed.
- The unsound candidate was removed. T-322 needs a contract revision that scopes
  namespace ownership/concurrency, or approval for a stronger platform-specific
  mechanism and narrower filesystem matrix, before implementation can proceed.
- 2026-08-14T13:35:41Z: A1-A2 require atomic wrong-leaf refusal, but Linux/macOS renameat/unlinkat mutate names without an expected retained-handle identity CAS; leaf and hard-link substitution remains possible between every check and mutation. Generic restart IDs also cannot prove non-reuse on all required filesystems. Revising concurrency scope or approving narrower platform/filesystem mechanisms is required.
- 2026-08-14T13:35:45Z: verification fail
