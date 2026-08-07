---
id: T-216-ship-digest-bound-existing-task-review
title: Ship digest-bound existing-task review
status: todo
priority: high
spec_ref: specs/v0.5.0.md#existing-task-review
dependencies:
    - T-161-apply-reviewed-task-bodies-with-compare-and-swap
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-215-add-the-generic-review-artifact-publisher
    - T-239-edit-exact-id-dependencies-safely
    - T-250-render-prompts-from-storage-neutral-context
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
updated_at: "2026-08-05T20:24:22Z"
---

# T-216-ship-digest-bound-existing-task-review Ship digest-bound existing-task review

## Description

Ship one advisory prompt and packaged skill for reviewing existing tracked tasks
without duplicating the four post-spec lenses or mutating task state directly.

## Acceptance

- `task-review` resolves one task/spec context, records the role-mandated v1
  source/template binding, and inventories related tasks and dependencies before
  judging the proposal.
- Findings cover outcome/spec alignment, overlap, dependency direction, acceptance, negative boundaries, evidence/oracles, operator gates, and unnecessary implementation prescription in one strict digest-bound schema.
- Final artifacts publish only while task/spec and prompt-template snapshots remain
  current. Accepted body changes route through `task author`; dependency edges use
  exact-ID add/remove; new outcomes use reviewed implicit-hold follow-ups.
- Todo tasks can continue into authoring; active, blocked, completed, and cancelled tasks remain reviewable but are not rewritten through the skill.
- Packaged/committed copies remain Agent Skills-compliant, provider-neutral, version-aware, and byte-identical.

## Verification Notes

- Use aligned, overlapping, cyclic, shallow-acceptance, unverifiable,
  over-prescribed, operator-gated, non-todo, stale-subject, wrong-role, and
  stale-replacement fixtures.
- Prove independent advisory output, generic publication, forbidden direct writers, package parity, and a reviewed `task author` handoff.

## Implementation Notes
