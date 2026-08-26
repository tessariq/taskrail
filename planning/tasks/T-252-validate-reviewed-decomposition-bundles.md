---
id: T-252-validate-reviewed-decomposition-bundles
title: Validate reviewed decomposition bundles
status: completed
priority: high
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies:
    - T-162-productize-digest-bound-post-spec-review-lenses
    - T-251-ship-the-outcome-focused-task-authoring-prompt
    - T-274-decode-reviewed-decomposition-bundles-strictly
updated_at: "2026-08-26T09:44:36Z"
completion_id: "dea4633b52ec7274a6a3b7c9bf5e7cd6"
last_verification_id: "760b619fa5b8a545fbe0d05e2327f653"
last_verification_result: pass
last_verified_at: "2026-08-26T09:44:36Z"
last_verified_completion_id: "dea4633b52ec7274a6a3b7c9bf5e7cd6"
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
- A4. Validation mechanically proves schema, binding, graph, trace, and review
  completeness only. A structurally valid bundle is not certified as semantically
  well-sized; that judgment belongs to T-251 and the reviewed decomposition flow.

## Verification Notes

- A1: complete one/two-pass positive bundles provide boundary-level evidence.
- A2: quote repetition, CRLF line, missing reverse edge, false anchor/dependency,
  and changed-final-byte fixtures prove each relation.
- A3: mutation corpus includes missing/malformed prompt bindings, snapshots the
  repository before every refusal, and confirms no task/state/review publication.
- A4: a shape-valid but semantically oversized fixture passes mechanical
  validation without emitting any semantic-size certification.

## Implementation Notes

- 2026-08-26T09:44:16Z: Validate reviewed decomposition bundles against complete post-spec evidence, live dependencies, fresh review context, and deferred-finding readiness gates.
- 2026-08-26T09:44:36Z: verification pass id 760b619fa5b8a545fbe0d05e2327f653 previous none completion dea4633b52ec7274a6a3b7c9bf5e7cd6
