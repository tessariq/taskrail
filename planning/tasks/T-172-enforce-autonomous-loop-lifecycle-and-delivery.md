---
id: T-172-enforce-autonomous-loop-lifecycle-and-delivery
title: Enforce autonomous loop lifecycle and delivery postflight
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-217-release-interrupted-active-work-safely
    - T-243-contain-autonomous-loop-process-trees
updated_at: "2026-08-08T08:40:49Z"
---

# T-172-enforce-autonomous-loop-lifecycle-and-delivery Enforce autonomous loop lifecycle and delivery postflight

## Description

Classify every child termination against frozen lifecycle, verification, Git
delivery, task-local loop policy, mutation, prompt/executable, lock, and process
evidence. Apply committed- or local-mode delivery postconditions and continue only
after a delivered, fresh completed pass; semantic review convergence remains a
prompt/skill contract rather than a fabricated postflight attestation.

## Acceptance

- Every child termination maps by exact precedence to one specified per-child
  outcome; only completed_pass with zero child exit may continue. Invocation
  success terminates as `no_work` or `iteration_limit`, with no-work taking
  precedence when the last completed task exhausted eligible work.
- Fresh success requires a preflight-absent matching verification ID/path and
  completion ID across every surface; stale/audit/partial-complete/block/rework/
  no-progress/child failure stop with exact safe recovery.
- Completed audit failure requires a fresh bound pass followed directly in the
  verification predecessor chain by a fresh fail in the same iteration; a
  completed fail without that chain is invalid postflight. Diagnostics identify the
  intermediate pass verification/binding separately from the final fail.
- Delivered recognized outcomes require clean tree, same full attached ref,
  unchanged frozen spec/config/layout/storage/review/prompt/executable bytes,
  unchanged pre-existing `loop_policy` and `loop_reason` fields, valid task
  policy, and no contained process; remote is not_checked. The final complete local
  ref namespace is unchanged except for the expected attached-branch advance; no
  unexpected ref change may survive postflight, and the dynamically enumerated
  uppercase root-ref-candidate set remains byte-identical with no new matching
  entry other than excluded `COMMIT_EDITMSG`.
- Committed delivery requires implementation plus generated task/state bytes in
  exactly one direct-child commit. Local completed-pass requires exactly one
  direct-child product commit; blocked/rework requires one only when product bytes changed, otherwise
  unchanged HEAD plus exact valid ignored lifecycle/verification bytes is valid.
  No local branch stages/commits Taskrail metadata or creates an empty delivery
  commit.
- Pre-existing non-selected task bytes and selected immutable content are
  protected; only canonical lifecycle fields/notes and any number of follow-ups
  proven by selected-task verify reports may change. Every new follow-up depends
  on the selected task, omits loop policy, and remains implicitly held.
- Final diagnostics always report outcome, child exit/signal, all identity
  before/after values, validation, Git/ref/HEAD, task-local policy source and
  values, storage mode/root, configured/effective review maximum and source,
  prompt/executable hashes, mutation/process violations, local commits, remote,
  and next action.
- Exact dry-run and result-file nested schemas define nullability, source enums,
  iteration counting, commit/violation ordering, and preflight refusal behavior
  without fabricating iteration-only evidence.
- T-244 owns external result-file path safety and publication of these diagnostics.
- Hook execution and semantic necessity are not fabricated postflight evidence;
  exact final commit identity/signature/provenance is enforced only where frozen
  repository policy provides a mechanical oracle, while prompt and T-218 evidence
  cover opaque child behavior.
- Transient ref/reflog movement is likewise prompt/manual-evaluation behavior:
  before/after snapshots prove final namespace equality, not that a child never
  moved and restored a ref or wrote a reflog entry.

## Verification Notes

- Map every outcome to setup/action/public diagnostic/filesystem+Git evidence;
  cover stale/mismatched IDs, full ref-namespace/ancestry/dirty/control/task mutation,
  bounded implicitly held follow-ups, frozen policy fields, iteration caps, and
  recovery-only verify.
- Persist manual success, hold, blocked, rework, partial-complete, audit-fail,
  external-writer, task-policy mutation, stale evidence,
  executable/prompt mutation, and containment-failure reports.

## Implementation Notes
