---
id: T-429-recover-loop-executable-staging-after-kill
title: Recover loop executable staging after a killed coordinator
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T17:35:05Z"
completion_id: "d4d3d511be9a6e734d6d5c3a97613f63"
last_verification_id: "9ebca2d047ba79190e198984ce5c08d4"
last_verification_result: pass
last_verified_at: "2026-10-31T17:35:05Z"
last_verified_completion_id: "d4d3d511be9a6e734d6d5c3a97613f63"
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

- 2026-10-31T17:34:55Z: Use a unique digest-prefixed executable stage per invocation; preserve exclusive creation and identity-checked cleanup. Document quiescent exact-path leftover recovery. Focused regression, full tests, vet, independent high-risk review and Linux process-kill QA passed; native Windows recovery not exercised.
- 2026-10-31T17:35:05Z: verification pass id 9ebca2d047ba79190e198984ce5c08d4 previous none completion d4d3d511be9a6e734d6d5c3a97613f63
