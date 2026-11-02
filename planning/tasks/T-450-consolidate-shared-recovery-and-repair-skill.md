---
id: T-450-consolidate-shared-recovery-and-repair-skill
title: Consolidate shared recovery and repair skill guidance
status: todo
priority: low
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies: []
updated_at: "2026-11-02T21:57:32Z"
---

# T-450-consolidate-shared-recovery-and-repair-skill Consolidate shared recovery and repair skill guidance

## Description

Adopter skill audit P11 identifies the duplicated inspect/dry-run/review/apply/
validate mechanical recovery flow in autonomous-recovery and taskrail-repair.
Both currently maintain that procedure separately, creating a drift risk.
taskrail-repair also owns specific generated-state merge-conflict guidance.

Give the common mechanical recovery procedure one canonical packaged owner
and have the other entry point delegate to it through an adopter-resolvable
skill reference. Preserve both installed skill identities and their current
scope; do not merge away entry points or broaden CLI recovery behavior.

## Acceptance

- One packaged owner defines the shared mechanical procedure; the other
  entry point explicitly delegates instead of maintaining a second recipe.
- Both entry points still lead to inspection, reviewed dry-run before apply,
  revalidation, and a stop for non-mechanical or ambiguous inconsistencies.
- Preserve freshness checks before state writers, the no-hand-edit and no-
  unrelated-implementation boundaries, and generated-state merge guidance.
- The reference resolves within the installed package in both supported skill
  layouts, without Taskrail-source-only paths or task/spec citations.
- Regenerate committed copies and retain package parity.

## Verification Notes

- Install the packaged skills in disposable adopter repositories for both
  supported layouts; follow each entry point and confirm its canonical
  procedure is reachable. Exercise mechanical drift and an ambiguous-state
  refusal; neither path may hand-edit statuses or invent work.
- Extend focused skill-contract coverage for delegation and retained safety
  boundaries rather than pinning duplicated prose. Run those tests, sandbox
  checks, and `task check:skills`; record transient evidence paths.

## Implementation Notes
