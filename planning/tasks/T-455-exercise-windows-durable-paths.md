---
id: T-455-exercise-windows-durable-paths
title: Exercise newly reachable Windows durable paths in tests
status: in_progress
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-322-provide-handle-bound-durable-filesystem-primitives
updated_at: "2026-11-25T13:02:19Z"
---

# T-455-exercise-windows-durable-paths Exercise newly reachable Windows durable paths in tests

## Description

Follow-up derived from T-322-provide-handle-bound-durable-filesystem-primitives's
durable-primitive boundary, discovered while fixing the native Windows
parent-directory barrier on branch `fix-windows-directory-barrier` (commit
`227a9a7`). That fix makes the production directory barrier succeed on local
NTFS: it reopens the retained directory handle with add-entry rights and
flushes it with `NtFlushBuffersFileEx`. Network and non-NTFS volumes still
refuse.

Native Windows CI run 45551585474 still did not exercise the newly reachable
durable paths, because two kinds of test gate hide them:

- Capability probes (`requireDirectoryDurability` in durabletx,
  `requireRecoveryDirectoryDurability` in taskrail) call Go's `os.File.Sync` on
  a read-only directory handle. That call is still denied on Windows, so the
  probes skip tests that production can now run.
- Hard-coded `runtime.GOOS == "windows"` skips claim "Windows reports directory
  durability as unsupported". That is no longer true on NTFS.

Outcome: on any filesystem where the production directory barrier succeeds,
tests of Replace, MoveDir, PublishDirectory, RemoveDir, durable transactions
and recovery run instead of skipping. They still skip where that barrier reports
`ErrUnsupported`.

## Acceptance

- Every durability capability probe in tests calls the production directory
  barrier (`durablefs.OpenExternal` + `SyncDir`, or the internal
  `syncDirectory` barrier inside durablefs). It skips only on
  `durablefs.ErrUnsupported`; any other probe error fails the test.
- No test skips with the rationale "Windows reports directory durability as
  unsupported". Those sites use the barrier probe instead.
- Legitimate platform, permission and case-sensitivity skips are unchanged.
- On native Windows NTFS, the durablefs, durabletx, reviewdir and taskrail
  tests for the paths above execute. Any failure they expose is reported, not
  hidden by weakening assertions. A production fix it requires, such as the
  legacy `FileRenameInformation` BOOLEAN use in `replaceStaged`, needs its own
  scope or contract decision.
- A native failure exposed this way is recorded in Implementation Notes and
  filed as a follow-up task (`verify --create-followup`) before this task
  completes; it is neither skipped nor asserted away here.

## Verification Notes

- `gofmt -l .`, `go vet ./...` for linux, windows and darwin, and Windows
  test-binary builds.
- Linux: the durablefs, durabletx and reviewdir tests (verbose), plus the
  taskrail tests.
- One approved native Windows CI run of the exact branch head. Record the
  verbose filesystem-step skips and which previously hidden tests now ran or
  failed.
- Ordinary-user and power-loss durability remain unestablished; they are out of
  scope here.

## Implementation Notes
