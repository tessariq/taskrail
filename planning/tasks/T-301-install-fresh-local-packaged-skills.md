---
id: T-301-install-fresh-local-packaged-skills
title: Install fresh local packaged skills transactionally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-247-install-packaged-skills-safely-in-local-mode
    - T-287-initialize-ignored-local-planning-storage-durably
updated_at: "2026-08-08T14:23:09Z"
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
