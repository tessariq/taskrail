---
id: T-166-publish-workflow-review-index-and-reports-with-cas
title: Derive canonical workflow review memory
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-165-maintain-bounded-workflow-adversarial-review
updated_at: "2026-08-26T09:35:12Z"
completion_id: "338cb982411fb670b8eeb4b27cd8c27b"
last_verification_id: "245413d3b4b30e6f4d5cc5558e70b93c"
last_verification_result: pass
last_verified_at: "2026-08-26T09:35:12Z"
last_verified_completion_id: "338cb982411fb670b8eeb4b27cd8c27b"
---

# T-166-publish-workflow-review-index-and-reports-with-cas Derive canonical workflow review memory

## Description

Purely derive canonical candidate workflow memory from one validated prior index
or absence and one validated immutable report. Durable pair publication remains
T-305.

## Acceptance

- A1. Given validated prior memory and report values, derivation replaces tested
  surfaces, applies only named finding transitions and freshness assessments,
  preserves unexplained rows by value, removes valid closures, and never reuses a
  finding number.
- A2. First-run absence and spec rollover produce the specified unresolved-finding
  and stale/fresh rows while preserving IDs and first-seen snapshots.
- A3. Output is canonical two-space JSON with exact field order, sorting, final LF,
  monotonic counter, unresolved findings only, and the 256 KiB/256-surface bounds;
  overflow refuses rather than dropping memory.
- A4. The report's `index_sha256_after` must equal the exact derived bytes. Prompt
  metadata remains report-local, and no caller-supplied candidate index is accepted.
- A5. Derivation is pure: it opens or writes no repository, review, task, state,
  spec, prompt, Git, or transaction path.

## Verification Notes

- A1-A4: transition-table goldens cover first run, replacement, unexplained rows,
  every finding status, rollover, monotonic allocation, ordering, and both caps;
  one-byte report/index digest mutations refuse.
- A5: filesystem sentinels and repeat invocations prove identical output with no
  side effects.

## Implementation Notes

- 2026-08-26T09:34:54Z: Derived workflow index now treats selected-spec path changes as freshness rollovers; focused and full checks pass.
- 2026-08-26T09:35:12Z: verification pass id 245413d3b4b30e6f4d5cc5558e70b93c previous none completion 338cb982411fb670b8eeb4b27cd8c27b
