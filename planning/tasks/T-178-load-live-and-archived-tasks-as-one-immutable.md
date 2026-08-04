---
id: T-178-load-live-and-archived-tasks-as-one-immutable
title: Load live and archived tasks as one immutable ledger
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-storage-and-unified-ledger
dependencies:
    - T-176-classify-persisted-and-legacy-task-identities
updated_at: "2026-08-04T23:06:23Z"
---

# T-178-load-live-and-archived-tasks-as-one-immutable Load live and archived tasks as one immutable ledger

## Description

Add the raw layout-3 live/archive physical index and combined loader used by
identity resolution and candidate migration. Command semantics and semantic
archive validity remain downstream.

## Acceptance

- Every configured relative root component enforces UTF-8/NFC, no
  dot/control/Windows-forbidden/trailing-dot-space/device values, and no
  Unicode-fold sibling alias; direct lower-case Markdown task entries reject
  deep/symlink/special/case variants and duplicate IDs/keys/filenames.
- Git discovery enumerates index entries and rejects sparse, missing,
  skip-worktree, assume-unchanged, or unavailable task content; empty archive
  root is optional.
- Storage derives only from path and loader records exact
  bytes/mode/mtime/path without reserialization.
- Candidate mode inspects source layouts/pre-existing archive without
  activating layout 3 or treating collisions as resolved.
- Loader returns complete claimant sets for allocator/resolver consumers
  deterministically.

## Verification Notes

- Map criteria to full root-component grammar, missing/empty/custom roots,
  index flags, aliases, deep/special entries, source candidates, and exact
  metadata.
- Prove no loader path mutates or silently omits indexed tasks.

## Implementation Notes
