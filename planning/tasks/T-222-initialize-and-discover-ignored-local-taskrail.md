---
id: T-222-initialize-and-discover-ignored-local-taskrail
title: Initialize and discover ignored local Taskrail storage
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-234-protect-repository-and-planning-writers
updated_at: "2026-08-05T22:04:16Z"
---

# T-222-initialize-and-discover-ignored-local-taskrail Initialize and discover ignored local Taskrail storage

## Description

Initialize and inspect personal Taskrail planning beneath `.taskrail/local/`
without making a non-adopting repository appear dirty. Implicit writer bootstrap
is owned by T-245.

## Acceptance

- Explicit local init durably writes layout-2 local config, specs, planning,
  notes, and strict runtime origin only after managed Git exclusions are effective;
  T-247 owns optional local skills.
- Plain `init --local` leaves `.agents/`, `.claude/`, and skill exclusions
  byte-for-byte unchanged; no assistant content is part of the default scaffold.
- Discovery preserves distinct worktree, Git/common-directory, storage-root, and
  logical-path identities from any descendant invocation and linked worktree.
- Tracked, staged, mixed-mode, aliased, symlink/reparse, special, conflicting, or
  ineffectively ignored destinations refuse without partial scaffold/exclusion
  changes.
- Text/JSON inspection reports origin/current Git snapshots, drift, logical and
  physical roots including prompts/runtime, exclusion scope, promotion readiness,
  and `git clean -x/-X` risk without exposing lock secrets.

## Verification Notes

- Use temporary ordinary/linked Git worktrees for explicit/read-only,
  collision, exclusion, descendant-cwd, branch-drift, and rollback observations.
- Snapshot both assistant roots and effective exclusions around plain local init.
- Snapshot worktree/index/exclude bytes before each refusal and retain CLI golden
  output proving visible `git status` remains clean after success.

## Implementation Notes
