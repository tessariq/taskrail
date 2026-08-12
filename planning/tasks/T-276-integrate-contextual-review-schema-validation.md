---
id: T-276-integrate-contextual-review-schema-validation
title: Integrate contextual review schema validation
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-274-decode-reviewed-decomposition-bundles-strictly
    - T-275-decode-workflow-adversarial-review-memory-strictly
updated_at: "2026-08-12T20:18:36Z"
---

# T-276-integrate-contextual-review-schema-validation Integrate contextual review schema validation

## Description

Provide the reusable contextual validator that binds structured final-review file
evidence from the strict workflow decoder to durable repository content. Publisher
adapters, prompt-resolution precedence, and historical review reads remain owned
by their dedicated tasks.

## Acceptance

- A1. One reusable contextual validator accepts already strictly decoded workflow
  file evidence and resolves product paths against the report's bound Git tree or
  final-review paths against immutable published review bytes.
- A2. Resolution requires a canonical repository-relative logical path, a regular
  blob, and an exact raw-byte SHA-256 match; lexical validity alone never passes.
- A3. The validator rejects active artifact, proposal, runtime, managed-state, and
  physical local-overlay paths, including canonical aliases into those roots.
- A4. Validation is side-effect-free and independent of prompt resolution and
  historical review reading, so publisher and reader tasks can compose it without
  granting lifecycle or publication authority.

## Verification Notes

- A1/A2: bind valid Git-tree and published-review evidence, then exercise missing
  entries, non-regular entries, digest mismatches, and paths absent from the bound
  Git tree even when they exist in the worktree.
- A3: exercise direct and aliased artifact/proposal/runtime/managed-state roots and
  physical local-overlay paths under committed and local storage contexts.
- A4: repository snapshots prove no files or tracked state change during validation;
  focused API tests keep prompt resolution, publication, and review reads outside
  this capability.

## Implementation Notes

- 2026-08-12T18:40:59Z: Acceptance requires surfaces this repository has not built: review publication preview/apply (T-215 todo), role-mandated prompt resolution and its invalid_proposal/prompt_invalid/source_changed precedence (T-159, T-236, T-255 todo), durable published-review roots and historical 'review show' (T-292, T-293, T-294 todo). machine_contract.go marks 'review publish', 'review show', and every 'prompt' command MachineOriginPlanned/MachineJSONAbsent, and cmd/taskrail/root.go registers no review or prompt command, so A1, A3, and A4 cannot be verified. Declared dependencies cover only the decoders (T-274, T-275), which leaves A2 the sole reachable criterion and any delivery an arbitrary slice. Operator must decide whether to re-sequence T-276 behind the publisher, prompt-resolution, and review-show tasks by adding those dependencies, or re-scope its acceptance to the context binding the existing decoders support.
- 2026-08-12T18:41:43Z: verification fail
- 2026-08-12: Operator approved rescoping this task to the reusable contextual file-evidence validator. T-215 and type adapters own publication integration, T-255 owns prompt-resolution precedence, and T-294 owns historical review reads.
- 2026-08-12T20:14:33Z: Operator approved narrowing T-276 to reusable contextual file-evidence validation; publication integration, prompt precedence, and historical reads remain in their owning tasks.
- 2026-08-12T20:18:30Z: Added side-effect-free contextual workflow file-evidence validation for exact bound Git blobs and no-follow immutable published reviews, with digest, root, alias, and worktree-drift coverage; go test ./..., go vet ./..., and taskrail validate pass.
- 2026-08-12T20:18:36Z: verification pass
