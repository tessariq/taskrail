---
id: T-173-check-cross-surface-workflow-contract-integrity
title: Derive and check the v0.5 workflow contract index
status: todo
priority: high
spec_ref: specs/v0.5.0.md#skill-and-prompt-behavioral-contract-tests
dependencies:
    - T-167-add-active-spec-scoped-statistics
    - T-315-promote-local-packaged-skills-with-explicit
    - T-225-prove-local-autonomous-delivery-across-git
    - T-246-classify-every-v0-5-writer-transaction-and
    - T-249-allow-explicit-skill-evaluation-release-waivers
    - T-256-retire-bootstrap-planning-reviews
    - T-258-retire-the-temporary-source-checkout-autonomous
updated_at: "2026-08-08T08:40:49Z"
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
  force-added metadata, unconsented promotion, storage-blind delivery, incidental
  private provenance, invented attribution, and Git identity/configuration edits.
  Committed custom planning directories and the fixed local overlay consume exact
  reported transient artifact paths. Frozen repository-visible policy governs
  generic Git conventions, only caller-owned instruction authorizes exposing local
  Taskrail identity/path provenance, and outcome-required product-byte cases remain
  positive exceptions rather than false failures.
- Cross-surface fixtures prove canonical lifecycle citations,
  review-artifact role-to-prompt mappings, template-binding and historical-read
  behavior, review/decomposition and lightweight SDD handoff compatibility,
  scoped lock delegation, task-local loop-policy refusal/preservation, and
  provider independence without recopying each feature's unit matrix.
- Arbitrary repository overrides are not represented as certified and no embedded
  workflow invokes a named model API/provider CLI. No durable binding contains a
  physical local path or claims prompt delivery, reviewer identity, or semantic quality.
- Deterministic checks certify embedded instructions and scripted observable
  outcomes only; the index assigns opaque real-agent path/provenance observations
  to T-218 and never presents them as credential-free attestation.
- README, AGENTS, workflow, skill-productization, human-notes, skill-evaluation,
  upgrade/recovery/review-evidence/SDD docs, and terse CHANGELOG entries contain
  one non-contradictory model with no stale repository-policy guidance.
- The temporary source-checkout loop has been retired completely; no runner,
  queue, prompt, harness, executable guidance, or packaged/release reference
  remains outside historical task/spec evidence.
- The derived index identifies portable versus platform-specific suites consumed
  by T-248; it does not itself change CI.

## Verification Notes

- Map criteria to registry/parity/drift tests, targeted mutation fixtures,
  wrong-role and stale-prompt fixtures, historical review reads, Agent Skills
  validation, loop-policy and SDD drift fixtures, documentation link/search
  assertions, provider scans, and the generated test-surface manifest.
- Run formatting, vet, full/race tests, cross-build, parity, task-body, validate,
  and freshness as integration evidence, not as substitutes for feature oracles.

## Implementation Notes
