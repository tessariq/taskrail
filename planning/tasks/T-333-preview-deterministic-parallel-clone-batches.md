---
id: T-333-preview-deterministic-parallel-clone-batches
title: Preview deterministic parallel clone batches
status: todo
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-308-publish-deterministic-loop-selection-and-dry-run
    - T-314-integrate-loop-continuation-and-terminal
updated_at: "2026-08-18T15:50:07Z"
---

# T-333-preview-deterministic-parallel-clone-batches Preview deterministic parallel clone batches

## Description

Extend loop dry-run into a deterministic, side-effect-free plan for one bounded
dependency-ready parallel frontier. Expose enough clone, workspace, delivery,
and task evidence for an operator to review exactly what local parallel execution
would launch without creating a workspace or weakening sequential defaults.

## Acceptance

- `loop --dry-run --parallel <n>` validates the complete parallel-only flag set,
  defaults clone depth to `1`, workspace root to the operating-system temporary
  root, retention to `failure`, and delivery to `local`, and rejects stale
  clone/delivery flags when effective parallelism is one.
- Planning supports committed Taskrail storage only, freezes the attached base
  ref/HEAD and invocation policy, and selects at most the effective width and
  remaining iteration budget from explicit active-spec allowances in ordinary
  deterministic rank. Every selected task is todo with completed dependencies;
  no selected task depends directly or transitively on another selected task.
- The preview validates an explicit workspace root no-follow outside repository,
  Git, managed-storage, result, and semantic-input roots. Clone depth accepts one
  positive integer or `full`; retention accepts only `never|failure|always`; and
  review delivery requires exactly one resolvable caller-owned adapter while
  local delivery rejects one.
- The uniform result emits the exact extended `LoopDryRunResult`, `ParallelPlan`,
  `ParallelWorkspace`, and ranked frontier shapes from the v0.5 machine companion.
  Sequential preview reports `parallel:null`; parallel preview reports every
  effective policy and exclusion reason without fabricating a selected singleton.
- Valid run/none and invalid report-result classifications remain consistent with
  the inherited dry-run contract. Every preview leaves worktree/index/refs,
  planning bytes, lock/runtime roots, result destinations, temporary directories,
  and child/adapter process counts unchanged.
- Root help, command documentation, README examples, changelog, strict decoders,
  machine inventory, and schema-drift tests describe the new flags and exact
  result union without claiming execution has shipped.

## Verification Notes

- Table-driven selection fixtures cover width/iteration caps, deterministic rank,
  independent held-task bypass, held or unfinished dependency isolation, no-work,
  and a frontier whose candidate tasks would conflict by path but remain
  schedulable because the DAG makes no file-disjointness claim.
- Flag/path fixtures cover defaults, duplicate/conflicting values, finite/full
  depth, explicit/default temporary roots, aliases, traversal, links, unsafe
  permissions, committed/local storage, adapter combinations, and sequential
  stale-intent refusal with complete before/after snapshots.
- CLI goldens and strict round trips cover exact text/JSON, nullability, ordering,
  warning/error classification, and zero workspace/ref/process effects on Linux,
  macOS, and native Windows where path behavior differs.

## Implementation Notes
