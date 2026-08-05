---
id: T-155-add-the-repository-mutation-lock-protocol
title: Add the repository mutation lock protocol
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-08-04T21:32:13Z"
---

# T-155-add-the-repository-mutation-lock-protocol Add the repository mutation lock protocol

## Description

Introduce one Git-common-directory mutation-lock protocol shared by linked
worktrees, with explicit ownership, refusal, stale recovery, and scoped
delegated-join primitives. Writer-family retrofit is a separate dependent task.

## Acceptance

- One exclusive lock coordinates linked worktrees and records command, PID, host,
  start time, repository identity, and only a delegation-token digest.
- A second owner refuses without mutation; same-process misuse and malformed
  metadata fail safely, and lock acquisition/release is interruption-aware.
- Abruptly abandoned locks are never auto-cleared; diagnostics provide
  inspect-and-remove recovery without claiming a distributed lease.
- Delegation tokens are unguessable, metadata exposes only their digest, and
  joining requires matching repository, executable identity, token, delegated
  writer capability, and task-field write set.
- Delegated ownership cannot widen its declared capabilities or task-field write
  set, and unsupported or unrelated delegated writes refuse before mutation.
- Read-only callers need no lock and the protocol is portable across supported
  operating systems.

## Verification Notes

- Map criteria to linked-worktree contention, malformed/stale metadata,
  interruption, token secrecy/mismatch, executable/repository mismatch, and
  native lock observations.
- Use portable process helpers rather than timing-only goroutine assertions.

## Implementation Notes
