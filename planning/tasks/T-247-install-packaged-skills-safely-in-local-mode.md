---
id: T-247-install-packaged-skills-safely-in-local-mode
title: Plan local packaged skill destinations safely
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-222-initialize-and-discover-ignored-local-taskrail
updated_at: "2026-08-08T08:40:49Z"
---

# T-247-install-packaged-skills-safely-in-local-mode Plan local packaged skill destinations safely

## Description

Produce a read-only, deterministic destination, exclusion, and ownership plan for
local packaged skills. Fresh installation and forced refresh consume this plan in
separate transactions.

## Acceptance

- A1. Given the active local context and embedded package, the planner inventories
  every file under both normal assistant discovery roots and exactly one owned
  exclusion per packaged-skill subtree, never an assistant/shared parent or local
  overlay destination.
- A2. Classification records destination presence, exact bytes/digest, marker
  state, tracked/staged/effective-ignore state, exclusion ownership/scope, aliases,
  case/Unicode collisions, entry type, and linked-worktree consequences.
- A3. The plan distinguishes safe fresh creation, safe explicit refresh, preserved
  parity mirrors, adopter-owned/ambiguous collisions, and compatible or conflicting
  marker states without writing skills, exclusions, config, index, or state.
- A4. Output order and ownership inventory are stable and sufficient for fresh
  install, refresh, status, and promotion consumers to recheck the same identities.

## Verification Notes

- A1-A3: ordinary and linked-worktree matrices cover absent, parity, stamped,
  legacy, dual, divergent, tracked, staged, ignored, aliased, colliding, and special
  destinations while snapshots prove no mutation.
- A4: deterministic inventory goldens are consumed unchanged by fresh-install,
  refresh, status, and promotion contract fixtures.

## Implementation Notes
