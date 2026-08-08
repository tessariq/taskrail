---
id: T-170-add-deterministic-autonomous-loop-preflight-and
title: Add deterministic autonomous loop preflight and dry-run
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-160-ship-the-lifecycle-complete-task-implementation
    - T-169-select-autonomous-work-through-policy-barriers
    - T-223-run-every-v0-5-command-against-local-storage
updated_at: "2026-08-08T08:40:49Z"
---

# T-170-add-deterministic-autonomous-loop-preflight-and Add deterministic autonomous loop preflight and dry-run

## Description

Implement the loop CLI contract, read-only task-local-policy selection, preflight,
override authorization, committed/local storage selection, implementation-review
budget resolution, and exact dry-run reporting before child execution. Exclude
the Taskrail source checkout explicitly while supporting installed adopters.

## Acceptance

- Parsing requires a child for execution, forbids one for dry-run, defaults max
  iterations to one, accepts only positive bounds, rejects execution JSON,
  accepts an optional positive per-child timeout, rejects retry/background
  options, and rejects ambiguous delimiter/flag forms.
- `--max-review-iterations` accepts only `1..5`, overrides the configured maximum
  without changing it, remains distinct from child `--max-iterations`, and is
  frozen in the rendered task prompt and diagnostics.
- Preflight requires a valid clean non-bare attached worktree, equal root,
  attached non-unborn HEAD, no in-progress task, layout 2, one valid committed or
  local storage context, and available shared lock. Local metadata must be
  effectively ignored, untracked, unstaged, and valid.
- Preflight snapshots the complete local `refs/*` namespace and dynamically
  enumerated uppercase root ref candidates in the worktree/common Git directories,
  excluding only `COMMIT_EDITMSG`. Repository policy and
  caller-owned provenance authorization remain semantic prompt/manual-evaluation
  context rather than a new parsed or mechanically frozen loop input.
- Selection matches read-only active ranking plus the exact task-local loop-policy
  semantics; held tasks are transparent unless they block an allowed candidate,
  and no candidate launches nothing with a clean `none` result.
- Dry-run emits the common envelope with exact result `action`, `reason`, nullable
  `selected_task`, non-null `tasks` and `violations`, nullable `prompt`, `git`,
  `lock`, `storage`, `review`, `execution`, and mode-specific `delivery`; warnings remain the
  envelope's non-null top-level array. Selected/task rows use
  only `task_id`, `status`, `active_spec`, `source`, `effective_policy`, `reason`,
  `eligible`, `held_dependencies`, and `disposition`; dry-run never mutates task
  or state bytes.
- Built-ins are inherently authorized. Replacements execute only with frozen exact
  template SHA authorization; absent/mismatch makes dry-run invalid and a supplied
  digest without a replacement is an argument error. Source-checkout execution is
  rejected by the exact repository predicate.

## Verification Notes

- Map criteria to CLI table tests including omitted/valid/invalid timeout, exact dry-run goldens,
  no-launch helper evidence, dirty/detached/unborn/bare/root/task/lock/task-policy
  cases, and override/source boundaries.
- Snapshot the complete managed semantic store plus visible Git index/status before
  and after every committed/local dry-run and refused execution branch; execution
  fixtures also snapshot all local refs and standard, arbitrary-name, and custom
  uppercase root ref candidates, including absent/present transitions, `EVIL_REV`,
  and alias refusal.

## Implementation Notes
