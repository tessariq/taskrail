---
id: T-433-refuse-result-files-aliasing-repository-by-identity
title: Refuse result files that alias the repository by identity
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T11:39:12Z"
---

# T-433-refuse-result-files-aliasing-repository-by-identity Refuse result files that alias the repository by identity

## Description

Bug (unconfirmed on macOS/Windows; inferred from code). The result-file
"outside the repository" check `loopResultWithin`
(internal/taskrail/loop_result_file.go) is a case-sensitive string prefix. On
case-insensitive APFS or NTFS, or with a different drive-letter case, a path
such as `/Users/me/REPO/out.json` or `c:\work\repo\.GIT\x.json` passes and the
file is written into the worktree or Git directory after postflight, dirtying
a repository already reported clean. The spec requires refusing alias parents
with `path_blocked`. Separately, a symlinked ancestor is refused with
"not a directory", which hides the real reason (macOS `/tmp` is a symlink). Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- Containment is decided by directory identity (or case-folded comparison where the filesystem is case-insensitive), refusing worktree, Git dir, common dir and managed-directory aliases with `path_blocked`.
- A symlinked ancestor is refused with a message naming the symlink.
- Tests cover a case-variant path where a case-insensitive filesystem is available and skip otherwise.

## Verification Notes

- CI on macOS and Windows runners with a case-variant result path.

## Implementation Notes
