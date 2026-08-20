---
id: T-338-guide-temporary-loop-orchestration-and-delivery
title: Guide temporary loop orchestration and delivery recovery
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-336-run-bounded-parallel-batches-in-the-temporary-loop
updated_at: "2026-08-20T11:58:47Z"
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
- A valid undelivered bundle is offered to the operator with the exact
  `scripts/autonomous-loop/run.sh --resume-delivery <absolute-bundle-path>` command
  only after explicit confirmation. The bridge never edits bundle bytes, relaunches
  an agent, repeats lifecycle transitions, resumes an in-progress outcome, clears
  a lock, resets/rebases/stashes, bypasses hooks, force-pushes, or automatically
  retries after refused resume. Successful resume is followed by source/remote
  identity checks and the same exact-head GitHub workflow wait.
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
- Task-body, queue, Taskrail validation, and retirement-reference checks prove the
  bridge is held, source-checkout-only, and removed with the temporary directory.

## Implementation Notes
