---
id: T-225-prove-local-autonomous-delivery-across-git
title: Prove local autonomous delivery across Git worktrees
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-245-cover-the-complete-implicit-local-bootstrap-matrix
updated_at: "2026-08-05T22:04:34Z"
---

# T-225-prove-local-autonomous-delivery-across-git Prove local autonomous delivery across Git worktrees

## Description

Prove the complete autonomous loop can deliver repository product changes while
Taskrail metadata stays local and ignored. Cover real Git/worktree boundaries and
the same lifecycle, iterative-review, verification, containment, and postflight
evidence required of committed mode.

## Acceptance

- Local preflight freezes mode/root, effective review maximum, prompt/executable,
  task ledger, attached ref/HEAD, index, exclusion, and visible cleanliness.
- A completed-pass child creates exactly one direct-child product commit, leaves no staged/committed
  Taskrail path, and records exact valid ignored completion/verification bytes;
  committed-mode fixtures retain their combined delivery requirement.
- Blocked, rework, completed-unverified, audit-fail, child/process failure, and
  local metadata mutation map to the specified outcomes and never continue to
  another task. Metadata-only blocked/rework may retain HEAD; changed product
  bytes require exactly one direct-child product commit, and no branch creates an empty delivery commit.
- Linked-worktree contention, branch drift, exclusion changes, mode/root changes,
  escaped descendants, and unrelated local ledger edits fail postflight with
  complete safe diagnostics.
- Final results report storage and review policy evidence alongside Git/lifecycle
  evidence without claiming remote delivery, reviewer identity, or mechanically
  observed review-iteration counts.

## Verification Notes

- Persist sandbox reports for committed/local success and every non-success
  outcome using real child helpers and real local commits on Linux, macOS, and
  native Windows where behavior is platform-specific.
- Assert exact visible Git diff/index/commit contents separately from ignored
  Taskrail state and rerun the full behavioral contract suite.

## Implementation Notes
