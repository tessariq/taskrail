---
id: T-336-run-bounded-parallel-batches-in-the-temporary-loop
title: Run bounded parallel batches in the temporary loop
status: todo
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-18T22:30:19Z"
---

# T-336-run-bounded-parallel-batches-in-the-temporary-loop Run bounded parallel batches in the temporary loop

## Description

Backport the bounded parallel batch shape onto the temporary source-checkout
loop so this repository's remaining v0.5 work can execute one dependency-ready
frontier per invocation while the product command is unimplemented. The batch
reuses the existing queue, prompt, child, commit-metadata, and delivery
contracts, stays opt-in, and never claims product acceptance.

## Acceptance

- `run.sh --parallel <n>` accepts a positive integer, defaults to `1`, composes
  with the existing backend, model, effort, timeout, and iteration flags, and is
  frozen for the invocation. Effective width `1` behaves exactly like the current
  sequential invocation, and parallel-only flags are rejected at width `1`.
- Width above `1` refuses before creating any workspace, clone, or child when the
  checkout is dirty, detached, bare, or its branch tip differs from `HEAD`, when
  planning storage is not committed, or when the existing repository, binary
  freshness, and queue preconditions fail.
- Selection reads the frozen validated queue in row order and takes at most the
  effective width and remaining iteration budget from `run` rows that are `todo`,
  have completed dependencies, and depend on no other selected row. Held rows and
  rows a child may not execute are never selected.
- `--dry-run --parallel <n>` prints the exact ordered frontier, effective width,
  frozen base ref/`HEAD`, workspace-root and clone-depth policy, retention policy,
  and the per-row reason every other open row was held or ineligible. It creates
  no workspace, clone, ref, process, or result file.
- Each selected task runs in one private clone beneath one invocation-private
  workspace root outside the worktree, Git directory, and planning storage,
  created with `--no-local --single-branch --no-tags --depth 1` unless
  `--clone-depth <positive|full>` overrides it, and proven not to be a
  hard-linked full-history clone. Each clone has its own Git common directory,
  and child argv, prompt, review policy, timeout, and both commit-message checks
  equal the sequential contract.
- No worker selects work, reaches the source checkout or another clone, or is
  retried. The first worker failure launches no replacement task and no new
  frontier; workers already launched are contained and allowed to finish, and
  their independent results stay eligible.
- Delivery is serial and local: one integration clone at the frozen base replays
  successful candidates in frontier order into one per-task commit each,
  re-projects `planning/STATE.md` through `taskrail repair --apply` instead of
  agent resolution, and permits exactly one bounded integration child per
  semantic conflict, which may not drop acceptance, delete a detecting test, or
  integrate another candidate.
- Publication runs the repository's full gate once over the final integration
  head, then re-verifies source cleanliness, attached ref, and base `HEAD`,
  performs one non-force fast-forward, and pushes to `origin/main`. Drift refuses
  without reset, checkout overwrite, rebase, stash, or partial publication.
- The terminal report names integrated and unpublished rows exactly, treats zero
  accepted candidates as a failed batch, retains failed workspaces under
  `--keep-workspaces never|failure|always` defaulting to `failure`, and keeps
  retained absolute paths out of committed state.
- Queue mutation stays parent-owned and post-batch: only exact fresh
  verification-created follow-ups are appended as `hold-operator`, after which
  the complete queue is revalidated, committed with its owning outcome, and
  frozen out of the invocation.
- `scripts/autonomous-loop/AGENTS.md` and the loop prompt document the opt-in
  batch, its refusals, and that it satisfies none of T-333, T-334, or T-335.

## Verification Notes

- `scripts/autonomous-loop/test.sh` gains fixtures for flag validation and
  freezing, precondition refusals, frontier selection and exclusion reasons,
  dry-run purity, clone shallowness and isolation, worker-failure containment,
  replay ordering, `STATE.md` re-projection, publication drift refusal, partial
  batch reporting, and post-batch queue append rules.
- Sandbox Git repositories under temporary directories drive real multi-worker
  batches with stub child executables covering all-pass, mixed, all-fail,
  timeout, and conflicting-replay outcomes, asserting source worktree, ref, and
  planning bytes before publication preflight.
- One real multi-task batch is recorded under
  `planning/artifacts/manual-test/T-336-run-bounded-parallel-batches-in-the-temporary-loop/<timestamp>/`
  with `plan.md` and `report.md`.

## Implementation Notes
