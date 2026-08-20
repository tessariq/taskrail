---
id: T-338-guide-temporary-loop-orchestration-and-delivery
title: Guide temporary loop orchestration and delivery recovery
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-336-run-bounded-parallel-batches-in-the-temporary-loop
updated_at: "2026-08-20T19:22:21Z"
completion_id: "9c29e21f7585d5a16fb85cb2fb33321f"
---

# T-338-guide-temporary-loop-orchestration-and-delivery Guide temporary loop orchestration and delivery recovery

## Description

Add a repository-local parent-agent operator guide beside the temporary
source-checkout loop. The bridge interactively configures and supervises the
existing script, makes worker integration and post-push CI observation explicit,
and safely picks up its private XDG delivery-recovery bundles without turning the
temporary mechanism into a packaged Taskrail skill. The complete bridge is removed
with `scripts/autonomous-loop/` when T-258 retires the bootstrap loop.

## Acceptance

- Operator guidance under `scripts/autonomous-loop/` elicits unresolved temporary
  runner choices including backend, model, effort, iteration budget, parallel
  width, timeout, clone depth, and workspace retention; verifies required local
  tools and binary freshness; runs the matching dry-run; explains the exact
  frontier, exclusions, frozen base, integration/delivery policy, and requires
  confirmation before one live invocation.
- The guide treats the calling parent agent as supervisor, not Git authority. It
  follows every ranked worker through terminal report validation, then observes
  the script-owned serial replay, mechanical `STATE.md` reprojection, bounded
  conflict child, aggregate gate, per-task commits, guarded fast-forward, and
  non-force push. It reports integrated and unpublished rows without manually
  selecting, entering a worker workspace, staging, cherry-picking, committing,
  repairing state, changing refs, pushing, replacing a worker, or retrying a
  failed child.
- After any pushed head, the bridge discovers and waits for every GitHub workflow
  attached to that exact commit, including applicable CI, Planning checks, and
  CodeQL runs. It reports pending/fail/cancelled/missing checks distinctly, never
  infers green from local tests or push success, and concludes success only when
  the requested workflows are terminal and successful and local `main`, `HEAD`,
  and `origin/main` agree.
- On non-zero exit or interruption, the bridge preserves and summarizes ignored
  coordinator/worker logs, retained workspaces, terminal lifecycle evidence, and
  safe next actions. When the runner reports an absolute private XDG recovery
  bundle, it identifies that exact bundle, inspects its complete/delivered marker,
  repository/task/outcome/base/report/message/candidate identities and current
  source preconditions, and explains that it resumes parent-owned delivery only;
  it never treats a retained workspace or free-form child output as a bundle.
- When the selected backend or operator reports quota exhaustion, the bridge
  labels that interpretation and any stated reset as attributed external,
  potentially heuristic evidence. It continues supervising the current runner so
  already-launched siblings drain and script-owned integration, delivery, and
  unpublished-candidate outcomes settle; it does not interrupt the runner,
  replace a worker, launch another frontier, mutate the queue, skip a gate, or
  reinterpret the ordinary terminal result.
- Quota handling preserves available coordinator, worker, and wrapper diagnostics
  under ignored artifacts or external storage, warns that provider output may be
  sensitive and incomplete, and reports preservation failure. The bridge quotes
  reset information with its source and supplied timezone/offset, never invents a
  reset instant, and never refunds or carries forward the current invocation's
  finite budget.
- A valid undelivered bundle is offered to the operator with the exact
  `scripts/autonomous-loop/run.sh --resume-delivery <absolute-bundle-path>` command
  only after explicit confirmation. The bridge never edits bundle bytes, relaunches
  an agent, repeats lifecycle transitions, resumes an in-progress outcome, clears
  a lock, resets/rebases/stashes, bypasses hooks, force-pushes, or automatically
  retries after refused resume. Successful resume is followed by source/remote
  identity checks and the same exact-head GitHub workflow wait.
- Execution after a reported reset is offered only through a new dry-run, fresh
  source and binary preflight, newly explicit finite budget, and fresh operator
  confirmation. It never uses `--resume-delivery` to relaunch an agent, resumes a
  worker/session or `in_progress` outcome, sleeps or schedules in the background,
  persists future launch intent, or automatically invokes the runner.
- `scripts/autonomous-loop/AGENTS.md` identifies the bridge as temporary,
  operator-owned, provider-specific only through caller choices, and outside the
  shipped skill package. Queue policy keeps this task `hold-operator`; ordinary
  loop workers cannot modify the guide or controls.
- T-258's retirement coverage is extended to require removal of the bridge,
  recovery-bundle guidance, tests, and every live reference before v0.5.0 release.
  No bridge content is copied into embedded or committed packaged skills; T-337
  owns the durable product replacement through product result files and
  `taskrail recover`.

## Verification Notes

- Shell/static fixtures cover complete and missing choices, tool/binary refusal,
  dry-run confirmation, exact one live invocation, ranked worker completion,
  clean/conflicting/partial integration, and assertions that the parent guide
  performs no runner-owned Git or planning mutation.
- Fake `gh` fixtures cover delayed discovery, pending-to-pass, fail, cancelled,
  missing and unrelated-head workflows, multiple applicable workflows, exact-head
  filtering, and no false green conclusion.
- Recovery fixtures cover valid complete/undelivered and delivered bundles,
  incomplete/tampered/wrong-repository/wrong-task/stale-base/dirty-source/refused
  bundles, explicit confirmation, successful resume without agent invocation,
  resume refusal without retry, and post-resume CI waiting. Bundle and workspace
  paths remain absent from committed state.
- Quota fixtures cover sequential backend failure, parallel failure with a
  successful sibling, sibling draining and ordinary partial integration, wrapper
  diagnostics, absent/malformed/relative/timezone-free reset text, preservation
  failure, exact attempt accounting, and a newly confirmed post-reset invocation.
  Negative assertions prove no interruption, queue mutation, replacement,
  delivery-resume misuse, budget refund, background wait, or automatic relaunch.
- Task-body, queue, Taskrail validation, and retirement-reference checks prove the
  bridge is held, source-checkout-only, and removed with the temporary directory.

## Implementation Notes

- 2026-08-20T19:21:59Z: Added the temporary parent-agent operator bridge with confirmed dry-run snapshot binding, one-shot runner supervision, ranked integration and gate reporting, stable exact-head GitHub workflow observation, strict delivery-only XDG recovery inspection, attributed quota accounting, and fixture/manual coverage; extended T-258 retirement scope.
- 2026-08-20T19:22:21Z: verification pass
