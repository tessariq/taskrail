---
id: T-306-ship-the-sandboxed-workflow-adversarial-review
title: Ship the sandboxed workflow adversarial review skill
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-297-ship-complete-storage-neutral-prompt-rendering
    - T-305-publish-workflow-review-reports-and-memory
updated_at: "2026-08-08T14:23:09Z"
---

# T-306-ship-the-sandboxed-workflow-adversarial-review Ship the sandboxed workflow adversarial review skill

## Description

Ship the provider-neutral packaged workflow-adversarial skill that performs one
bounded sandboxed review and publishes only validated report/memory evidence.

## Acceptance

- A1. The skill reads the selected spec and prior `INDEX.json` through Taskrail,
  treats only exact `review_not_found` as first-run memory, renders the built-in
  workflow prompt, and chooses at most three rotated surfaces with a fresh angle.
- A2. It requires a clean attached source snapshot, captures HEAD/spec/product
  digests, runs mutating probes only in an isolated sandbox, records terminal
  observable evidence, cleans up, and refuses a clean claim when source bytes or
  disallowed outputs remain.
- A3. The skill stages exactly one strict transient `report.json`, handles stale
  rows and finding dispositions conservatively, and invokes the public workflow
  publisher with JSON and exact snapshots; it never writes durable review files directly.
- A4. The workflow is report-only and advisory: it never edits product, spec,
  tasks, status, loop policy, verification, or Git history and never promotes a finding.
- A5. Packaged source and committed mirrors are Agent Skills-compliant,
  storage-neutral, provider-independent, marker-free, and byte-identical; installed
  copies follow nested version metadata.

## Verification Notes

- A1-A4: sandbox scenarios cover first/subsequent runs, rotation, clean/finding/
  inconclusive probes, dirty source, cleanup failure, stale memory, every
  disposition, publisher refusal, and forbidden-write sentinels in both storage modes.
- A5: skill conformance, package parity, decoy logical-path, command-existence, and
  provider-independence checks plus one reproducible manual two-run review.

## Implementation Notes
