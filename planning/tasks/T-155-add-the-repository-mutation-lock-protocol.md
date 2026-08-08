---
id: T-155-add-the-repository-mutation-lock-protocol
title: Add the repository mutation lock protocol
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies: []
updated_at: "2026-08-08T08:40:49Z"
---

# T-155-add-the-repository-mutation-lock-protocol Add the repository mutation lock protocol

## Description

Introduce one mutation-lock protocol using the Git common directory for matching
worktrees and the configured root-local runtime directory outside Git. Provide
ownership, refusal, and delegated-join primitives; operator lock recovery and
writer-family integration are separate dependent outcomes.

## Acceptance

- One exclusive lock coordinates linked worktrees or one discovered non-Git root
  and records the exact normative metadata including random lock ID, executable
  digest when delegated, and only a delegation-token digest.
- A second owner refuses without mutation; same-process misuse and malformed
  metadata fail safely, and lock acquisition/release is interruption-aware.
- Abruptly abandoned locks are never auto-cleared; T-231 owns operator inspection
  and guarded clearing without claiming a distributed lease.
- Delegation tokens are unguessable, metadata exposes only their digest, and
  joining requires matching repository, executable identity, token, delegated
  writer capability, and task-field write set.
- Delegated ownership cannot widen its declared capabilities or task-field write
  set, and unsupported or unrelated delegated writes refuse before mutation.
- Read-only callers need no lock and the protocol is portable across supported
  operating systems.
- Local-mode ownership uses the same logical repository identity as committed
  mode while keeping storage-root identity explicit; mixed-mode or root-mismatch
  joins refuse rather than sharing an ambiguous writer.
- Primitive tests exercise explicitly supplied committed and local repository
  contexts. T-222/T-223 retain storage discovery and end-to-end command routing,
  so this foundational outcome does not depend on local initialization shipping.

## Verification Notes

- Map criteria to linked-worktree contention, malformed/stale metadata,
  interruption, token secrecy/mismatch, executable/repository mismatch, and
  native lock observations across explicitly supplied committed/local contexts.
- Use portable process helpers rather than timing-only goroutine assertions.

## Implementation Notes
