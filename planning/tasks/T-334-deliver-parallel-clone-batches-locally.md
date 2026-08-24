---
id: T-334-deliver-parallel-clone-batches-locally
title: Deliver parallel clone batches locally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-333-preview-deterministic-parallel-clone-batches
    - T-314-integrate-loop-continuation-and-terminal
    - T-244-publish-streamed-loop-results-out-of-band
updated_at: "2026-08-18T15:50:12Z"
---

# T-334-deliver-parallel-clone-batches-locally Deliver parallel clone batches locally

## Description

Deliver one explicitly requested ready frontier through concurrent isolated
shallow-clone workers and one serial local integration clone. Preserve one
authoritative Taskrail history by mechanically replaying valid task outcomes,
using one bounded integration-agent attempt only for semantic conflicts, and
publishing the verified aggregate through one guarded fast-forward.

## Acceptance

- Parallel execution freezes T-333's plan, creates one private clone per selected
  task under the default temporary or explicit workspace root, and uses
  `--no-local --single-branch --no-tags` plus depth `1` by default (or the exact
  configured positive depth/full history). It proves local finite-depth clones
  are actually shallow and never silently hard-link a full object store.
- Each clone has an independent Git common directory and Taskrail lock. The same
  caller argv receives one fresh selected-task implementation prompt, exact
  frozen review/timeout/executable policy, and only that worker's delegated
  lifecycle/follow-up capability. Workers run concurrently up to effective width,
  never invoke `next`, and are individually contained and streamed without output
  corruption.
- A worker succeeds only with zero exit, valid `completed_pass`, one direct-child
  candidate commit from the frozen base, clean clone, expected refs, exact task/
  verification evidence, and no surviving process. Any other inherited outcome
  fails that worker; no failure is retried, no replacement or new frontier is
  launched, and already-running siblings are drained to independent results.
- One integration clone considers successful candidates in deterministic
  selection order. It replays one task commit at a time, accepts no unrelated
  mutation, and reprojects aggregate `STATE.md` only through `taskrail repair
  --apply`; every resulting commit keeps implementation, tests, docs, selected
  terminal task bytes, and the current state projection together.
- A semantic replay conflict launches exactly one fresh integration child bound
  to the evolving head, candidate, relevant task/spec contracts, worker evidence,
  and conflict. It cannot discard acceptance or detecting tests, alter planning
  relationships/policy, hand-edit state, integrate another candidate, or retry.
  Unresolved integration fails only that candidate and does not suppress later
  dependency-independent successful candidates.
- A fresh aggregate integration child checks the exact final candidate after all
  accepted tasks. Final delivery rechecks source cleanliness, attached ref/base
  HEAD, index, relevant refs/config/planning inputs, then performs one non-force
  fast-forward. Drift or aggregate failure leaves the source checkout unchanged;
  no reset, stash, overwrite, partial publication, or automatic repair occurs.
- All independently valid candidates publish even when siblings fail. Exact batch
  pass/partial/fail diagnostics report ranked worker, integration, workspace,
  commit, cleanup, and safe-next-action evidence; partial/fail exits non-zero and
  launches no more work. Retention defaults to failed workspaces only and cleanup
  removes only invocation-owned identity-matching roots. Cleanup is final
  postflight before the immutable result-file publication: removed workspaces are
  null, retained failed workspaces keep exact paths, cleanup failure is terminal
  evidence, and publication failure never recreates already-cleaned workspaces.
- Parallel execution rejects local ignored Taskrail storage, source-checkout use,
  recursive-submodule requirements, and linked-worktree workers before cloning.
  Sequential `--parallel 1` behavior remains unchanged. README, command docs,
  workflow guidance, changelog, prompts, and strict machine contracts describe
  the exact boundary and do not advertise unbounded workers or retries.

## Verification Notes

- Real temporary-repository tests run two and three ready tasks with deliberately
  different completion order, proving bounded overlap, deterministic integration,
  independent locks, default/configured clone depth, full-history opt-in, private
  default/configured roots, exact prompt transport, and cleanup policy.
- Worker matrices cover pass, block, rework, timeout, signal, malformed delivery,
  dirty clone, extra commit/ref/process, sibling failure, and no-refill behavior;
  partial batches prove every valid independent success still publishes.
- Retention/result matrices cover never/failure/always, worker and integration
  cleanup refusal, null-after-cleanup diagnostics, retained exact paths, and final
  publication failure after cleanup without workspace recreation.
- Integration matrices cover clean replay, mechanical `STATE.md` conflict repair,
  semantic product/test conflict closed by one agent attempt, unresolved conflict,
  later independent acceptance, aggregate-test failure, source drift at every
  final preflight boundary, and exact one-commit-per-task fast-forward history.
- Persist sandbox manual-test plans/reports for multi-worker local delivery and
  partial success, then run race-enabled focused tests, full tests, vet, build,
  Taskrail validation, skill parity, and task-body hygiene on supported platforms.

## Implementation Notes
