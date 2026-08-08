---
id: T-302-refresh-local-packaged-skills-safely
title: Refresh local packaged skills safely
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-247-install-packaged-skills-safely-in-local-mode
    - T-301-install-fresh-local-packaged-skills
updated_at: "2026-08-08T14:23:09Z"
---

# T-302-refresh-local-packaged-skills-safely Refresh local packaged skills safely

## Description

Refresh an existing local packaged-skill installation only through explicit
`init --with-skills --force`, preserving ownership and Git invisibility.

## Acceptance

- A1. Refresh is applicable only to an initialized local repository with an
  explicit forced skill request; repeating `--local`, omitting `--force`, or using
  implicit bootstrap cannot refresh installed bytes.
- A2. The command consumes and rechecks T-247's refresh plan across both roots,
  preserves marker-free parity mirrors, normalizes compatible legacy/matching-dual
  markers to nested-only, and refuses divergent, conflicting, or adopter-owned bytes.
- A3. Skill replacements and required existing exclusions publish as one durable
  transaction; no refresh path removes an owned exclusion or makes skills visible,
  and snapshot drift/failure cannot leave mixed versions.
- A4. Init/status results report exact refreshed/preserved files and exclusions in
  deterministic order, and the resulting inventory remains suitable for later
  explicit promotion.

## Verification Notes

- A1/A2: applicability and marker-state matrices cover current, legacy, dual,
  parity, divergent, conflicting, missing, and adopter-owned destinations.
- A3/A4: cross-root/exclusion races and fault recovery prove all-or-none refresh,
  clean Git visibility, deterministic results, and unchanged promotion ownership.

## Implementation Notes
