---
id: T-302-refresh-local-packaged-skills-safely
title: Refresh local packaged skills safely
status: completed
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-247-install-packaged-skills-safely-in-local-mode
    - T-301-install-fresh-local-packaged-skills
updated_at: "2026-08-25T21:20:57Z"
completion_id: "3d03c2e67935f26e929ad2549e09a85f"
last_verification_id: "06a4795d89ca50b3d3b0304f50e42a61"
last_verification_result: pass
last_verified_at: "2026-08-25T21:20:57Z"
last_verified_completion_id: "3d03c2e67935f26e929ad2549e09a85f"
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

- 2026-08-25T21:18:38Z: Added transactional forced local skill refresh with ownership and drift checks.
- 2026-08-25T21:20:57Z: verification pass id 06a4795d89ca50b3d3b0304f50e42a61 previous none completion 3d03c2e67935f26e929ad2549e09a85f
