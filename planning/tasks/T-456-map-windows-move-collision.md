---
id: T-456-map-windows-move-collision
title: Report Windows no-replace move collisions as exists
status: in_progress
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-322-provide-handle-bound-durable-filesystem-primitives
updated_at: "2026-11-25T22:09:43Z"
---

# T-456-map-windows-move-collision Report Windows no-replace move collisions as exists

## Description

Follow-up discovered by T-455-exercise-windows-durable-paths. Native Windows
CI run 45620924637 at `ac43517` failed
`TestPublishDirectoryRollsBackAliasCreatedAtCommit` with `PublishDirectory =
Object Name already exists., want alias or native no-replace refusal`.

The Windows no-replace directory move (`moveDirectoryNoReplace` in
`internal/durablefs/move_windows.go`) correctly refuses to rename onto an
existing name, including a case alias, but returns the raw
`STATUS_OBJECT_NAME_COLLISION` from `NtSetInformationFile`. That value does not
satisfy `errors.Is(err, os.ErrExist)`, unlike the Linux `renameat2` and macOS
`renameatx_np` refusals, so callers lose the typed no-replace refusal.

Outcome: a Windows no-replace move collision reports as `os.ErrExist`, with the
native refusal and every other error unchanged.

## Acceptance

- `moveDirectoryNoReplace` on Windows still refuses to replace an existing
  destination name; only the collision status is reported as an error that
  satisfies `errors.Is(err, os.ErrExist)`. Other NT statuses are returned
  unchanged.
- `TestPublishDirectoryRollsBackAliasCreatedAtCommit` passes on native Windows
  NTFS, reaching its no-exact-name and alias-preservation assertions; no
  assertion is weakened.
- Linux and macOS behavior is unchanged.

## Verification Notes

- Regression evidence: the failing native test from run 45620924637.
- `gofmt -l .`, `go vet ./...` for linux, windows and darwin, Windows test
  binary builds, Linux tests.
- One approved native Windows CI run of the exact branch head, with the
  verbose filesystem step showing the test passing.

## Implementation Notes
