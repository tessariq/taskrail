---
id: T-170-add-deterministic-autonomous-loop-preflight-and
title: Add autonomous loop invocation and repository preflight
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-160-ship-the-lifecycle-complete-task-implementation
    - T-169-select-autonomous-work-through-policy-barriers
    - T-223-run-every-v0-5-command-against-local-storage
updated_at: "2026-08-08T08:40:49Z"
---

# T-170-add-deterministic-autonomous-loop-preflight-and Add autonomous loop invocation and repository preflight

## Description

Establish the loop invocation parser and immutable repository-preflight snapshot
used by dry-run and execution. Resolve invocation budgets, Git and storage context,
and the complete control-input read set without selecting work, authorizing a
replacement prompt, launching a child, or writing managed state.

## Acceptance

- Execution requires exactly one child argument vector after `--`; dry-run rejects
  a child; execution rejects `--json`; and omitted, duplicate, misplaced, or
  ambiguous delimiter/flag forms fail before repository access. Unsupported
  retry or background forms are rejected rather than ignored.
- `--max-iterations` defaults to `1` and accepts only positive integers.
  `--max-review-iterations` accepts only `1..5` and resolves independently from
  the child count without changing configuration. `--timeout` accepts only a
  positive Go duration and omission remains an unlimited per-child deadline.
- Repository preflight requires Git, a valid clean non-bare worktree with attached
  non-unborn HEAD, equal Taskrail/worktree logical roots, layout 2, no existing
  `in_progress` task, an available shared lock, and exactly one valid committed or
  local storage context. Local managed paths are proven ignored, untracked,
  unstaged, valid, and unmixed; source-checkout execution is rejected by the exact
  `Taskfile.yml` plus `internal/toolchain/cmd/freshcheck` predicate.
- The resulting immutable snapshot records all task/state/spec/config/layout and
  prompt inputs, storage mode/root, configured/effective review budget and source,
  timeout, attached ref/HEAD, index/status, complete local `refs/*`, verification
  IDs/artifact set, and direct regular uppercase root-ref candidates from the
  worktree and common Git directories. Enumeration is no-follow, rejects aliases
  and special files, and excludes only `COMMIT_EDITMSG`.
- Repository policy and caller-owned provenance authorization remain opaque
  semantic context: preflight does not invent parsed policy fields or claim that
  before/after snapshots can detect transient ref or reflog movement.

## Verification Notes

- Map parser criteria to table tests proving accepted argv and every rejected
  arity, delimiter, bound, timeout, JSON, retry, and background form without
  repository reads or writes.
- In temporary committed/local repositories, assert exact preflight facts for
  clean, dirty, detached, unborn, bare, unequal-root, in-progress, lock-held,
  mixed-storage, invalid-local, and source-checkout cases.
- Snapshot managed bytes, index/status, all local refs, and standard, arbitrary,
  custom, absent, aliased, and special uppercase root candidates such as
  `EVIL_REV`; prove this foundation performs no child launch or managed write.

## Implementation Notes
