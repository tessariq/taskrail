---
id: T-247-install-packaged-skills-safely-in-local-mode
title: Install packaged skills safely in local mode
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-222-initialize-and-discover-ignored-local-taskrail
updated_at: "2026-08-06T13:46:30Z"
---

# T-247-install-packaged-skills-safely-in-local-mode Install packaged skills safely in local mode

## Description

Support opt-in local skill installation with destination-specific exclusions while
preserving adopter-owned content and marker-free committed parity mirrors.

## Acceptance

- A1. `init --local --with-skills` installs only Taskrail-owned destinations after
  tracked/staged/collision/no-follow checks and adds exact managed exclusions.
- A2. Marker-free byte-identical mirrors are preserved as mirrors; installed copies
  receive nested version metadata and conflicting/divergent copies refuse.
- A3. Local promotion moves skills only with `--with-skills`; otherwise their exact
  exclusions and local bytes remain.

## Verification Notes

- A1: ordinary/linked worktree matrices verify ownership and narrow exclusions.
- A2: nested/legacy/dual/missing/divergent marker fixtures prove classification.
- A3: promotion preview/apply snapshots prove opt-in behavior and parity.

## Implementation Notes
