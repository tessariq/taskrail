---
id: T-173-check-cross-surface-workflow-contract-integrity
title: Derive and check the v0.5 workflow contract index
status: todo
priority: high
spec_ref: specs/v0.5.0.md#skill-and-prompt-behavioral-contract-tests
dependencies:
    - T-167-add-active-spec-scoped-statistics
    - T-224-promote-local-taskrail-state-into-committed
    - T-225-prove-local-autonomous-delivery-across-git
    - T-246-classify-every-v0-5-writer-transaction-and
    - T-249-allow-explicit-skill-evaluation-release-waivers
updated_at: "2026-08-04T21:32:13Z"
---

# T-173-check-cross-surface-workflow-contract-integrity Derive and check the v0.5 workflow contract index

## Description

Add only the cross-surface registry, drift, provider-boundary, documentation, and
packaged-smoke checks that feature tasks cannot prove in isolation.
Feature-specific behavior remains owned and verified by its implementation task.

## Acceptance

- Registry-by-construction and byte parity cover every embedded prompt/skill;
  Agent Skills conformance covers metadata and nested resources; delineated
  command/lifecycle fixtures reject missing, invented, or misordered behavior
  rather than matching unrelated prose.
- Schema-inventory and skill-invocation fixtures reject missing JSON support,
  prose-parsed results, wrong outer generations, unregistered shapes, and mixed
  streamed/result output.
- Local skill fixtures reject default or implicit installation, overlay-prefixed
  discovery paths, broad assistant exclusions, direct logical-path opens,
  force-added metadata, unconsented promotion, and storage-blind delivery.
- Cross-surface fixtures prove canonical lifecycle citations,
  review/decomposition and lightweight SDD handoff compatibility, scoped lock
  delegation, task-local loop-policy refusal/preservation, and provider
  independence without recopying each feature's unit matrix.
- Arbitrary repository overrides are not represented as certified and no embedded
  workflow invokes a named model API/provider CLI.
- README, AGENTS, workflow, skill-productization, human-notes, skill-evaluation,
  upgrade/recovery/review-evidence/SDD docs, and terse CHANGELOG entries contain
  one non-contradictory model with no stale repository-policy guidance.
- The derived index identifies portable versus platform-specific suites consumed
  by T-248; it does not itself change CI.

## Verification Notes

- Map criteria to registry/parity/drift tests, targeted mutation fixtures,
  Agent Skills validation, loop-policy and SDD drift fixtures, documentation
  link/search assertions, provider scans, and the generated test-surface manifest.
- Run formatting, vet, full/race tests, cross-build, parity, task-body, validate,
  and freshness as integration evidence, not as substitutes for feature oracles.

## Implementation Notes
