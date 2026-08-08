---
id: T-276-integrate-contextual-review-schema-validation
title: Integrate contextual review schema validation
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-274-decode-reviewed-decomposition-bundles-strictly
    - T-275-decode-workflow-adversarial-review-memory-strictly
updated_at: "2026-08-08T14:23:08Z"
---

# T-276-integrate-contextual-review-schema-validation Integrate contextual review schema validation

## Description

Integrate the strict task, spec, decomposition, and workflow decoders with the
active repository context so preview and apply reject unsafe or stale review
evidence while historical review reads remain stable.

## Acceptance

- A1. Review publication preview and apply use the same strict decoder for each
  artifact role and apply the same path, digest, identity, cap, reference, and
  prompt-resolution checks before writing.
- A2. Structured final-review file evidence resolves to a matching regular blob in
  the bound Git tree or immutable published review and rejects active transient,
  proposal, runtime, managed-state, and physical local-overlay paths.
- A3. Prompt role/contract defects, invalid current replacements, and changed valid
  resolutions retain the specified `invalid_proposal`, `prompt_invalid`, and
  `source_changed` precedence.
- A4. Historical `review show` decodes published bytes without re-evaluating them
  against a later active transient root or prompt resolution.

## Verification Notes

- A1: run every review type through aligned preview/apply fixtures and compare
  validation observations before publication.
- A2: bind valid Git-tree and published-review evidence, then exercise aliases,
  missing blobs, transient roots, proposal/runtime roots, and local overlays.
- A3: perturb artifact role, current replacement validity, source, and template
  bytes in precedence order and inspect exact error codes.
- A4: change active storage/prompt context after publication and verify historical
  reads remain byte-stable while new publication uses the new context.

## Implementation Notes
