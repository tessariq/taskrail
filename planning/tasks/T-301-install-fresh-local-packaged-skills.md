---
id: T-301-install-fresh-local-packaged-skills
title: Install fresh local packaged skills transactionally
status: completed
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-247-install-packaged-skills-safely-in-local-mode
    - T-287-initialize-ignored-local-planning-storage-durably
updated_at: "2026-08-25T20:21:35Z"
completion_id: "e5ee429446a95ae3d08ad9b311685aae"
last_verification_id: "511e7644913cf39a3ac60f2dd5fd1338"
last_verification_result: pass
last_verified_at: "2026-08-25T20:21:35Z"
last_verified_completion_id: "e5ee429446a95ae3d08ad9b311685aae"
---

# T-301-install-fresh-local-packaged-skills Install fresh local packaged skills transactionally

## Description

Install packaged skills only during explicit fresh local initialization, sharing
the scaffold's durable transaction and recovery boundary.

## Acceptance

- A1. Only `init --local --with-skills` on an absent layout selects fresh local
  installation; plain local init and implicit bootstrap leave both assistant roots
  and skill exclusions byte-for-byte unchanged.
- A2. The command consumes and rechecks T-247's complete safe-create plan, writes
  installed copies at normal `.agents` and `.claude` discovery paths, and creates
  exactly one effective owned exclusion per packaged-skill subtree.
- A3. Local scaffold, installed skill files, and exclusions are one durable
  candidate: collision, snapshot drift, write fault, interruption, or recovery
  never yields a logically accepted partial installation.
- A4. Installed copies use nested version metadata; embedded/parity mirrors remain
  marker-free sources. Init JSON reports every skill and exclusion deterministically,
  and ordinary Git status remains clean.

## Verification Notes

- A1/A2: fresh explicit/plain/implicit init fixtures verify selection, exact
  destinations, narrow exclusions, marker output, and JSON inventory.
- A3/A4: destination/exclusion races and fault injection across scaffold and skill
  publication prove recovery, no adopter-content overwrite, package parity, and a
  clean visible worktree.

## Implementation Notes

- 2026-08-25T20:21:15Z: Installed fresh local packaged skills transactionally with recovery checks.
- 2026-08-25T20:21:35Z: verification pass id 511e7644913cf39a3ac60f2dd5fd1338 previous none completion e5ee429446a95ae3d08ad9b311685aae
