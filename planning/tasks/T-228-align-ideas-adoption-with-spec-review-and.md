---
id: T-228-align-ideas-adoption-with-spec-review-and
title: Align IDEAS adoption with spec review and decomposition
status: todo
priority: high
spec_ref: specs/v0.6.0.md#human-owned-ideas-inbox
dependencies:
    - T-227-draft-reviewed-spec-changes-from-configured-ideas
    - T-162-productize-digest-bound-post-spec-review-lenses
    - T-164-orchestrate-adversarial-spec-to-task-decomposition
updated_at: "2026-08-05T22:04:57Z"
---

# T-228-align-ideas-adoption-with-spec-review-and Align IDEAS adoption with spec review and decomposition

## Description

Integrate IDEAS with Taskrail's existing spec authoring, post-spec review, and
decomposition skills without inventing a second planning lifecycle. After a
successful reviewed spec adoption, the agent maintains the free-form source
semantically and work proceeds through the normal spec-to-task boundary.

## Acceptance

- The packaged workflow treats IDEAS as rough context, drafts a coherent spec
  change, runs the existing spec review/disposition contract, and decomposes only
  an approved final spec.
- Direct task drafting is labeled exceptional, requires an already approved real
  spec anchor, and never bypasses task authoring/review or grants loop allowance.
- After successful spec adoption the agent proposes a reviewed IDEAS edit that
  moves or annotates adopted material with its spec path/anchor; the binary never
  guesses ranges or writes the source.
- Rejected, deferred, failed, or partial proposals leave source material available
  and do not claim adoption. Duplicate detection, per-entry receipts, and
  bidirectional synchronization are not claimed.
- Embedded/committed skills retain package parity and the workflow remains
  provider-neutral and LLM-free in the binary.

## Verification Notes

- Evaluate heading-, list-, and prose-heavy sources through accepted, rejected,
  deferred, conflict, and exceptional direct-task scenarios with exact spec/task/
  IDEAS snapshots.
- Run skill contract/parity checks and manually inspect the complete
  ideas-to-reviewed-spec-to-decomposition handoff without provider integration.

## Implementation Notes
