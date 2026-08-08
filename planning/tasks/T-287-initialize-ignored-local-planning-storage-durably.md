---
id: T-287-initialize-ignored-local-planning-storage-durably
title: Initialize ignored local planning storage durably
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-222-initialize-and-discover-ignored-local-taskrail
    - T-234-protect-repository-and-planning-writers
updated_at: "2026-08-08T14:23:08Z"
---

# T-287-initialize-ignored-local-planning-storage-durably Initialize ignored local planning storage durably

## Description

Implement explicit `init --local` as a durable creation of ignored layout-2
planning storage and strict origin metadata. This task owns scaffold/exclusion
publication; optional packaged skills and implicit writer bootstrap remain with
T-247 and T-245.

## Acceptance

- Init classifies every managed destination and the effective Git exclusion store
  before writes, installs one uniquely marked exclusion block, then proves all
  local managed paths ignored, untracked, and unstaged before semantic publication.
- One durable transaction publishes the strict local config, fixed specs/planning
  scaffold, NOTES template when absent, and strict runtime `origin.json`; recovery
  fences interruption before or during any exclusion/scaffold phase.
- Plain `init --local` leaves `.agents/`, `.claude/`, skill bytes, and skill
  exclusions unchanged and reports empty skill collections.
- Tracked/staged/mixed destinations, aliases, symlink/reparse traversal, special
  files, ineffective ignore rules, or concurrent changes refuse without a partial
  scaffold or exclusion edit.
- Success leaves ordinary Git status clean, preserves logical configured paths in
  semantic files, and returns the exact common init result and local storage mode.

## Verification Notes

- Run fresh local init in ordinary and linked worktrees and assert exact scaffold,
  origin schema, exclusion scope, result writes, and clean status/index.
- Snapshot assistant roots and exclusions around plain init and every collision or
  mixed-state refusal.
- Interrupt and fault each durable phase, then exercise shared recovery and compare
  originals/candidates without manual cleanup.

## Implementation Notes
