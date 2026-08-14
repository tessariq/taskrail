---
id: T-322-provide-handle-bound-durable-filesystem-primitives
title: Provide handle-bound durable filesystem primitives
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-222-initialize-and-discover-ignored-local-taskrail
    - T-317-bind-delegated-grants-to-the-owner-s-declared-task
updated_at: "2026-08-14T15:19:18Z"
---

# T-322-provide-handle-bound-durable-filesystem-primitives Provide handle-bound durable filesystem primitives

## Description

Provide the native filesystem substrate required by durable transactions: bind
managed paths to stable identities without following links, publish and replace
relative to retained directory handles, and make file and directory durability
barriers explicit on Linux, macOS, and Windows. This task owns primitives and
their failure model, not transaction phases or recovery policy. The repository
lock serializes Taskrail writers; external namespace mutation is detected by
no-follow traversal and immediate byte/mode/identity checks but is not claimed to
be atomically excluded by operating systems that mutate names rather than handles.

## Acceptance

- A1. Supported platforms open and classify existing ancestors and leaf entries
  without following symlinks or reparse points, retain a stable identity, and
  refuse aliases, pre-existing hard-link ambiguity, special entries, and observed
  identity substitution.
- A2. Handle-relative create, no-replace publication, replacement, removal, and
  directory creation stay beneath retained ancestors. Immediately before a
  name-based mutation, current bytes, mode, link count, and identity must match the
  bound candidate; disagreement refuses without mutation. The contract does not
  claim atomic compare-and-mutate against an external actor racing that final
  check outside Taskrail's repository lock.
- A3. File-content, metadata, and parent-directory durability barriers expose a
  precise success or classified unsupported/failure result; success never means
  only that buffered bytes reached an API boundary.
- A4. Restart-time checks rebind through no-follow ancestors and compare persisted
  bytes, mode, identity, and applicable volume/mount evidence. Identity evidence
  is an additional substitution signal, not a proof that an absent file identifier
  can never be reused; recovery policy requires semantic snapshots as its oracle.
- A5. Linux, macOS, and Windows use native implementations behind one narrow
  standard-library-style interface; `golang.org/x/sys` is the only approved new
  runtime dependency, and unsupported filesystems refuse rather than weakening
  guarantees.

## Verification Notes

- A1-A2: substitute ancestors and leaves before every observable operation
  boundary and assert no out-of-root mutation; verify that disagreement present at
  the final check refuses, and document the explicitly excluded post-check external
  namespace race.
- A3: inject each file and directory sync failure and prove success is reported
  only after the complete ordered barrier sequence.
- A4: close and reopen fixtures, including replaced ancestors and leaves, and
  compare persisted semantic snapshots plus identity evidence with rebound state.
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
- 2026-08-14T13:38:22Z: Operator approved a portable contract: Taskrail writers remain repository-lock serialized; retained ancestors, no-follow traversal, and immediate semantic/identity checks detect substitution, while atomic exclusion of hostile external post-check namespace races is explicitly out of scope.
- 2026-08-14T15:19:10Z: Implemented lock-bound native durable filesystem primitives with no-follow identity binding, handle-relative CAS mutations, ordered sync barriers, restart snapshots, cross-platform builds, focused/race/full tests, native sandbox, and bounded review; hostile external namespace races after the final check remain explicitly outside the contract.
- 2026-08-14T15:19:18Z: verification pass
