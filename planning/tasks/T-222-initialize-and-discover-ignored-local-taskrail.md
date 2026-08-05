---
id: T-222-initialize-and-discover-ignored-local-taskrail
title: Initialize and discover ignored local Taskrail storage
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-155-add-the-repository-mutation-lock-protocol
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-05T22:04:16Z"
---

# T-222-initialize-and-discover-ignored-local-taskrail Initialize and discover ignored local Taskrail storage

## Description

Initialize personal Taskrail planning beneath `.taskrail/local/` without making a
non-adopting repository appear dirty. Support deliberate `init --local` and the
same safe bootstrap from stateful commands while preserving every read-only
command's no-write contract.

## Acceptance

- Explicit local init writes layout-2 local config, specs, planning, and notes
  only after a managed Git exclusion is effective; T-223 owns optional local
  skill installation on the completed storage context.
- A syntactically valid stateful command may bootstrap local mode and reports that
  fact through the exact common warning on success or later semantic refusal;
  argument/Git/path refusal and every read-only command write nothing.
- Discovery preserves distinct worktree, Git/common-directory, storage-root, and
  logical-path identities from any descendant invocation and linked worktree.
- Tracked, staged, mixed-mode, aliased, symlink/reparse, special, conflicting, or
  ineffectively ignored destinations refuse without partial scaffold/exclusion
  changes.
- Text/JSON inspection uses the exact local status/path result schemas to report
  mode, logical/physical roots, exclusion scope, branch snapshot, promotion
  readiness, and `git clean -x/-X` risk without exposing lock secrets.

## Verification Notes

- Use temporary ordinary/linked Git worktrees for explicit/implicit/read-only,
  collision, exclusion, descendant-cwd, branch-drift, and rollback observations.
- Snapshot worktree/index/exclude bytes before each refusal and retain CLI golden
  output proving visible `git status` remains clean after success.

## Implementation Notes
