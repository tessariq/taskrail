---
id: T-225-prove-local-autonomous-delivery-across-git
title: Prove local autonomous delivery across Git worktrees
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-244-publish-streamed-loop-results-out-of-band
    - T-245-cover-the-complete-implicit-local-bootstrap-matrix
updated_at: "2026-08-08T08:40:49Z"
---

# T-225-prove-local-autonomous-delivery-across-git Prove local autonomous delivery across Git worktrees

## Description

Prove the complete autonomous loop can deliver repository product changes while
Taskrail metadata stays local and ignored. Cover real Git/worktree boundaries and
the same lifecycle, broad-round/final-diff review, verification, containment, and postflight
evidence required of committed mode.

## Acceptance

- Local preflight freezes mode/root, effective broad review-round maximum,
  including configured default `1` or explicit override `2`, the three-reviewer
  ceiling, conditional final-diff rule, prompt/executable,
  task ledger, attached ref/HEAD, complete local ref namespace, dynamic uppercase root ref candidates,
  index, exclusion, and visible cleanliness.
- A completed-pass child creates exactly one direct-child product commit, leaves no staged/committed
  Taskrail path, and records exact valid ignored completion/verification bytes;
  the commit tree and Git-visible provenance contain no incidental ignored
  task/spec/review/verification identifiers, managed paths, storage details, or
  invented attribution. Frozen repository-visible policy governs generic Git
  conventions, but only caller-owned instruction outside managed planning can
  authorize exposing a local Taskrail identity/path in commit metadata;
  outcome-required product bytes are distinct from that authorization;
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
  observed broad-round, reviewer, or final-diff-review counts.
- Scripted prompt/skill fixtures preserve caller-owned author/committer/signing
  configuration and do not instruct hook bypass. Postflight rejects unexpected
  local ref changes and enforces identity/signature/provenance only where frozen
  visible policy supplies an exact oracle; it does not claim to attest hook
  execution or arbitrary child tools.

## Verification Notes

- Persist sandbox reports for committed/local success and every non-success
  outcome using real child helpers and real local commits on Linux, macOS, and
  native Windows where behavior is platform-specific.
- Assert exact visible Git diff/index/commit contents separately from ignored
  Taskrail state, inspect commit subject/body/trailers and identity separately,
  exercise generic visible policy, caller-authorized identity/path provenance,
  outcome-required product bytes, and delayed/current self-authorization refusal,
  then rerun the full behavioral contract suite including out-of-band terminal
  result publication.

## Implementation Notes
