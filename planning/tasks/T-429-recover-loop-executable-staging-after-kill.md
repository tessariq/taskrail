---
id: T-429-recover-loop-executable-staging-after-kill
title: Recover loop executable staging after a killed coordinator
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T11:39:10Z"
---

# T-429-recover-loop-executable-staging-after-kill Recover loop executable staging after a killed coordinator

## Description

Bug. After the loop coordinator is killed with SIGKILL, every later loop run
with the same binary fails with
`stage loop executable .../taskrail/executables/<sha256>: file exists`, reported
as `repository_invalid` with no remedy. `lock clear` succeeds but removes only
the lock record, and nothing else cleans the staged copy.

Cause: `stageLoopExecutable` (internal/taskrail/loop_ownership.go) creates a
hash-named file with `O_EXCL` under the Git common directory; only a normal
shutdown removes it. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- After an abandoned stage and `lock clear`, a new loop invocation with the same binary stages and launches successfully.
- Cleanup still removes only the invocation's own staged copy; a concurrent invocation's copy is never removed.
- The staged-copy leftovers are discoverable and removable through a documented, guarded recovery path.

## Verification Notes

- Unit: stage twice in the same Git common directory without cleanup; the second stage succeeds.
- Manual: kill -9 a running loop, `lock clear`, rerun.

## Implementation Notes
