---
id: T-222-initialize-and-discover-ignored-local-taskrail
title: Discover repository and active storage context
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-05T22:04:16Z"
---

# T-222-initialize-and-discover-ignored-local-taskrail Discover repository and active storage context

## Description

Discover one managed repository and construct an explicit committed/local storage
context from any supported invocation directory. This task owns root identity,
layout classification, logical-to-physical mapping, and mixed-state refusal;
durable local initialization and inspection commands are separate slices.

## Acceptance

- Discovery preserves distinct managed root, worktree root, Git directory/common
  directory, storage root, lock root, and logical repository path identities from
  descendant invocations and linked worktrees.
- Layout-2 `storage_mode` selects committed physical roots or the fixed
  `.taskrail/local/{specs,planning,prompts}` overlay while persisted and reported
  semantic identities remain in configured logical namespaces.
- The context exposes canonical physical config, specs, planning, prompts,
  artifacts, and runtime paths plus repository identity needed by lock and
  transaction callers; callers never prepend the local overlay themselves.
- Missing/ambiguous roots, worktree/config mismatch, incompatible layout, aliases,
  symlink/reparse or special traversal, and mixed committed/local state refuse with
  no writes and never fall back to committed paths.
- Applicable non-Git ancestor discovery remains supported for committed mode;
  Git-required local behavior returns a classified capability refusal.

## Verification Notes

- Use table-driven ordinary Git, linked-worktree, custom-directory, descendant-cwd,
  and non-Git fixtures to assert every context identity and mapping.
- Add decoy committed/local roots, aliases, case collisions, and traversal entries
  to prove deterministic refusal and no physical fallback.
- Snapshot the worktree, index, and Git metadata around discovery-only success and
  refusal to prove this foundation is read-only.

## Implementation Notes
