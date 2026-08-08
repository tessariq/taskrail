---
id: T-247-install-packaged-skills-safely-in-local-mode
title: Install packaged skills safely in local mode
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-222-initialize-and-discover-ignored-local-taskrail
updated_at: "2026-08-08T08:40:49Z"
---

# T-247-install-packaged-skills-safely-in-local-mode Install packaged skills safely in local mode

## Description

Support explicit opt-in local skill installation at normal assistant discovery
paths with destination-specific exclusions, complete preflight, and transactional
publication while preserving adopter-owned content and marker-free committed
parity mirrors.

## Acceptance

- A1. Plain `init --local` and every implicit bootstrap leave assistant roots and
  skill exclusions untouched. Fresh `init --local --with-skills` is the only local
  install opt-in; initialized local repositories refresh through explicit
  `init --with-skills --force` without repeating `--local`.
- A2. Local copies are materialized under `.agents/skills/<packaged-skill>/` and
  `.claude/skills/<packaged-skill>/`, never `.taskrail/local/`. Each owned subtree
  receives one exact effective exclusion; parent assistant or shared skill
  directories are never excluded.
- A3. Before any write, installation classifies every destination and checks
  tracked/staged state, effective ignores, aliases, case/Unicode collisions,
  no-follow traversal, special files, and linked-worktree consequences. Any
  ambiguous or adopter-owned collision refuses the complete operation.
- A4. Fresh local scaffold, skill destinations, and exclusions publish under one
  durable recovery boundary. Destination changes after preflight, publication
  faults, or cleanup faults cannot leave a partially accepted installation.
- A5. Marker-free byte-identical mirrors are preserved as mirrors; installed
  copies receive nested version metadata, compatible legacy markers normalize
  only on forced refresh, and conflicting/divergent copies refuse unchanged.
- A6. Init JSON reports every installed file, while local status reports the exact
  managed skill-subtree exclusions in both assistant roots; ordinary committed installs
  report no exclusions, and ordinary Git status remains clean after local install
  or refresh.
- A7. The installer exposes exact destination ownership, digest, and exclusion
  inventory for T-224's promotion transaction. No init or refresh path removes a
  managed local skill exclusion or makes the installed files commit-visible.

## Verification Notes

- A1-A4: ordinary/linked worktree matrices snapshot assistant roots, index,
  exclusions, local scaffold, transaction state, and fault recovery.
- A5: nested/legacy/dual/missing/divergent marker fixtures prove classification.
- A6-A7: init/status snapshots and a promotion-handoff fixture prove clean Git
  visibility, complete typed inventory, unchanged ownership evidence, and parity;
  T-224 owns promotion preview/apply behavior.

## Implementation Notes
