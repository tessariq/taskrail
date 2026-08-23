---
id: T-313-validate-committed-and-local-loop-delivery-shapes
title: Validate committed and local loop delivery shapes
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-312-enforce-frozen-input-and-ledger-mutation-integrity
updated_at: "2026-08-23T18:16:57Z"
completion_id: "1c7bd4d57c2145f268b632642ed901b9"
last_verification_id: "2a8e8bf725721c9257495af7adbb35d5"
last_verification_result: pass
last_verified_at: "2026-08-23T18:16:57Z"
last_verified_completion_id: "1c7bd4d57c2145f268b632642ed901b9"
---

# T-313-validate-committed-and-local-loop-delivery-shapes Validate committed and local loop delivery shapes

## Description

Validate the final visible Git delivery shape for each recognized lifecycle
candidate in committed and local storage, using T-312's integrity result. Keep
remote delivery explicitly unchecked and reject unrelated or metadata-leaking
commits without performing Git repair.

## Acceptance

- Every delivered recognized lifecycle outcome has a clean visible tree, the same
  full attached ref, descendant HEAD, no surviving contained process, valid
  repository state, and no T-312 integrity violation. Remote delivery is always
  exactly `not_checked`.
- In committed mode, `head_after` is exactly one direct-child commit of
  `head_before`; that commit contains the complete implementation plus allowed
  generated selected-task/state/follow-up bytes and no unrelated mutation.
  Unchanged, merge, intermediate, multiple, dirty, or partial delivery is invalid.
- In local mode, `completed_pass` requires exactly one direct-child product commit
  when product bytes changed and forbids Taskrail metadata in index or commit.
  Local blocked/rework requires that product commit only when product bytes
  changed; otherwise unchanged HEAD is valid with exact ignored lifecycle and
  verification bytes.
- Local delivery rejects an empty metadata-only commit, staged/committed local
  Taskrail identity or provenance, workflow-created refs, and any incidental
  managed-path exposure. Outcome-required product content and mechanically
  enforceable frozen repository policy remain the only visible-content oracles.
- Delivery reports the exact ref, before/after HEAD, clean/descendant facts, and
  oldest-to-newest full commit IDs. Invalid delivery contributes
  `invalid_postflight`; child-failure dirt may be reported as evidence but is
  never accepted as delivered.

## Verification Notes

- Temporary committed repositories cover exact direct-child success and unchanged,
  merge, multiple, intermediate, dirty, unrelated, omitted-metadata, and partial
  commit shapes with tree-level assertions.
- Local repositories cover product change/no-change across completed, blocked,
  rework, and child-failure outcomes, including empty commits, staged/committed
  metadata, provenance leakage, created refs, and exact ignored lifecycle bytes.
- Exact diagnostic goldens prove commit ordering, full object IDs, same-ref and
  ancestry classification, remote `not_checked`, and no automatic reset, commit,
  push, merge, or repair.

## Implementation Notes

- 2026-08-23T18:16:47Z: Added read-only committed and local loop delivery validation with Git-shape fixtures.
- 2026-08-23T18:16:57Z: verification pass id 2a8e8bf725721c9257495af7adbb35d5 previous none completion 1c7bd4d57c2145f268b632642ed901b9
