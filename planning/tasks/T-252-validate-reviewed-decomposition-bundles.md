---
id: T-252-validate-reviewed-decomposition-bundles
title: Validate reviewed decomposition bundles
status: todo
priority: high
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies:
    - T-162-productize-digest-bound-post-spec-review-lenses
    - T-251-ship-the-outcome-focused-task-authoring-prompt
    - T-240-implement-the-normative-review-schema-decoders
updated_at: "2026-08-06T13:46:30Z"
---

# T-252-validate-reviewed-decomposition-bundles Validate reviewed decomposition bundles

## Description

Strictly validate the complete reviewed decomposition handoff before the durable
ImportDraft v2 writer performs any repository mutation.

## Acceptance

- A1. V2 draft, trace, immutable review passes, manifest, final spec-review binding,
  role-mandated review prompt bindings, reviewed bodies, and every raw digest
  decode through the shared strict schemas.
- A2. Unique exact quote or one-based line sources, bidirectional requirement/task
  keys, real anchors/dependencies, final-byte review, and complete dispositions hold.
- A3. Any stale digest, unknown/missing field, unreviewed change, bad pass sequence,
  wrong prompt role/contract/source shape, deferred high/medium finding, or 1 MiB
  violation fails before writes.

## Verification Notes

- A1: complete one/two-pass positive bundles provide boundary-level evidence.
- A2: quote repetition, CRLF line, missing reverse edge, false anchor/dependency,
  and changed-final-byte fixtures prove each relation.
- A3: mutation corpus includes missing/malformed prompt bindings, snapshots the
  repository before every refusal, and confirms no task/state/review publication.

## Implementation Notes
