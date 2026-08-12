---
id: T-276-integrate-contextual-review-schema-validation
title: Integrate contextual review schema validation
status: blocked
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-274-decode-reviewed-decomposition-bundles-strictly
    - T-275-decode-workflow-adversarial-review-memory-strictly
updated_at: "2026-08-12T18:41:43Z"
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

- 2026-08-12T18:40:59Z: Acceptance requires surfaces this repository has not built: review publication preview/apply (T-215 todo), role-mandated prompt resolution and its invalid_proposal/prompt_invalid/source_changed precedence (T-159, T-236, T-255 todo), durable published-review roots and historical 'review show' (T-292, T-293, T-294 todo). machine_contract.go marks 'review publish', 'review show', and every 'prompt' command MachineOriginPlanned/MachineJSONAbsent, and cmd/taskrail/root.go registers no review or prompt command, so A1, A3, and A4 cannot be verified. Declared dependencies cover only the decoders (T-274, T-275), which leaves A2 the sole reachable criterion and any delivery an arbitrary slice. Operator must decide whether to re-sequence T-276 behind the publisher, prompt-resolution, and review-show tasks by adding those dependencies, or re-scope its acceptance to the context binding the existing decoders support.
- 2026-08-12T18:41:43Z: verification fail
