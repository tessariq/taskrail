---
id: T-368-expose-selected-spec-digests-in-machine-results
title: Expose selected spec digests in machine results
status: completed
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-162-productize-digest-bound-post-spec-review-lenses
updated_at: "2026-08-28T08:09:24Z"
completion_id: "08fb625f22657c3e7c68a8b0361ea17c"
last_verification_id: "b02f7e73976e330a9a327e0544064a90"
last_verification_result: pass
last_verified_at: "2026-08-28T08:09:24Z"
last_verified_completion_id: "08fb625f22657c3e7c68a8b0361ea17c"
---

# T-368-expose-selected-spec-digests-in-machine-results Expose selected spec digests in machine results

## Description

Expose the exact selected spec byte identity through `spec show --json` so
storage-neutral digest-bound workflows never reconstruct a physical path or
re-hash ambiguously decoded output.

## Acceptance

- `SpecShowResult` includes one non-empty lower-case `sha256` of the exact selected
  raw spec bytes in committed and local storage, with or without `--anchors`.
- Text output remains exact spec content and all existing JSON fields retain their
  semantics and ordering under schema version 1.
- Post-spec review prompt and publication fixtures consume the reported digest
  without opening a logical or local-overlay path directly.

## Verification Notes

- Compare reported digests with exact committed/local fixture bytes, including
  CRLF and non-ASCII content, and run strict decoder/golden mutation tests.
- Run focused spec/prompt/review tests, full tests, vet, and cross-platform CI.

## Implementation Notes

- 2026-08-28T08:09:07Z: Expose exact selected-spec SHA-256 values in spec show machine results and preserve raw plain-text output.
- 2026-08-28T08:09:24Z: verification pass id b02f7e73976e330a9a327e0544064a90 previous none completion 08fb625f22657c3e7c68a8b0361ea17c
