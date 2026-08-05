---
id: T-173-check-cross-surface-workflow-contract-integrity
title: Check cross-surface workflow contract integrity
status: todo
priority: high
spec_ref: specs/v0.5.0.md#skill-and-prompt-behavioral-contract-tests
dependencies:
    - T-164-orchestrate-adversarial-spec-to-task-decomposition
    - T-166-publish-workflow-review-index-and-reports-with-cas
    - T-167-add-active-spec-scoped-statistics
    - T-172-enforce-autonomous-loop-lifecycle-and-delivery
    - T-202-ship-the-lightweight-sdd-handoff-skill
updated_at: "2026-08-04T21:32:13Z"
---

# T-173-check-cross-surface-workflow-contract-integrity Check cross-surface workflow contract integrity

## Description

Add only the cross-surface registry, drift, provider-boundary, documentation, and
packaged-smoke checks that feature tasks cannot prove in isolation.
Feature-specific behavior remains owned and verified by its implementation task.

## Acceptance

- Registry-by-construction and byte parity cover every embedded prompt/skill;
  Agent Skills conformance covers metadata and nested resources; delineated
  command/lifecycle fixtures reject missing, invented, or misordered behavior
  rather than matching unrelated prose.
- Cross-surface fixtures prove canonical lifecycle citations,
  review/decomposition and lightweight SDD handoff compatibility, scoped lock
  delegation, task-local loop-policy refusal/preservation, and provider
  independence without recopying each feature's unit matrix.
- Arbitrary repository overrides are not represented as certified and no embedded
  workflow invokes a named model API/provider CLI.
- README, AGENTS, workflow, skill-productization,
  upgrade/recovery/review-evidence/SDD docs, and terse CHANGELOG entries contain
  one non-contradictory model with no stale repository-policy guidance.
- Packaged native smoke evidence from feature tasks is complete for Linux, macOS,
  and Windows, and this task verifies the matrix and release-facing navigation.

## Verification Notes

- Map criteria to registry/parity/drift tests, targeted mutation fixtures,
  Agent Skills validation, loop-policy and SDD drift fixtures, documentation
  link/search assertions, provider scans, and the collected native smoke manifest.
- Run formatting, vet, full/race tests, cross-build, parity, task-body, validate,
  and freshness as integration evidence, not as substitutes for feature oracles.

## Implementation Notes
