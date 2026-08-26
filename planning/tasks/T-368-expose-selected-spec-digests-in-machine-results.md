---
id: T-368-expose-selected-spec-digests-in-machine-results
title: Expose selected spec digests in machine results
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-162-productize-digest-bound-post-spec-review-lenses
updated_at: "2026-08-26T09:03:47Z"
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
