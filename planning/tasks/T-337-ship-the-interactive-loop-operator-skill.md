---
id: T-337-ship-the-interactive-loop-operator-skill
title: Ship the interactive loop operator skill
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-244-publish-streamed-loop-results-out-of-band
    - T-335-deliver-parallel-batches-through-review-adapters
    - T-326-unblock-the-operator-lock-surface-under-a-retained
updated_at: "2026-08-20T11:57:50Z"
---

# T-337-ship-the-interactive-loop-operator-skill Ship the interactive loop operator skill

## Description

Ship a provider-neutral `taskrail-loop` skill that interactively configures,
previews, confirms, invokes, and supervises one bounded `taskrail loop` run. The
skill is the operator-facing parent agent: it explains and observes worker,
integration, delivery, CI, and recovery outcomes while the Taskrail coordinator
remains the sole authority that selects work, integrates candidate commits,
reprojects state, updates refs, and publishes results.

## Acceptance

- The packaged Agent Skill elicits only unresolved invocation choices: sequential
  or parallel execution, iteration and review budgets, width, timeout, opaque
  caller-owned child argv/model options, clone/workspace/retention policy, local or
  review delivery, review adapter, and the operator's CI requirement. It names
  safe defaults and stops rather than inventing a missing provider command,
  credential decision, adapter, or destructive authorization.
- Before execution, the skill runs the exact `taskrail loop --dry-run --json`
  corresponding to those choices, explains the selected task or ordered frontier,
  every exclusion, frozen base, storage/review/execution policy, integration order,
  delivery and CI boundary, and requires explicit confirmation of that plan. No
  mutating loop invocation, recovery apply, or external write occurs before
  confirmation.
- Execution allocates a safe caller-owned destination outside the repository for
  `--result-file`, launches the coordinator exactly once, and supervises streamed
  output plus the terminal envelope without treating free-form child output as
  authoritative. It never hides a retry, replacement worker, second frontier, or
  changed invocation policy.
- The parent skill agent reports every worker in deterministic rank with its
  lifecycle, candidate, containment, verification, and retained-workspace result;
  then supervises the coordinator's serial integration of accepted worker results.
  It confirms one commit per accepted task, mechanical `STATE.md` reprojection,
  bounded conflict-child outcomes and affected checks, aggregate integration-child
  and full-gate results, final integrated head, and every unpublished candidate.
  The skill itself never selects a task, enters worker scope, cherry-picks, stages,
  commits, merges, repairs state, changes refs, or pushes: those remain coordinator
  or explicit review-adapter responsibilities.
- Delivery supervision confirms the verified aggregate and published identities.
  Local delivery requires the coordinator's aggregate build/test gate and clean
  guarded fast-forward, and reports remote CI as `not_checked` unless the caller
  explicitly supplies a separate read-only CI observer. Review delivery monitors
  adapter-reported checks until terminal, requires `checks: pass` before claiming
  a merge is green, and treats `fail`, `unknown`, timeout, drift, or adapter
  refusal as non-success without bypass, force push, inferred approval, or retry.
- On every exit, including non-zero and interruption, the skill reads the result
  file when present and reports its safe next action, worker/integration retention,
  delivery state, and lifecycle evidence. For product transaction recovery it
  inspects `taskrail lock status --json`; when the retained transaction is blocked
  by that exact owner, it previews with `taskrail recover <transaction-id>
  --take-over-lock <lock-id> --expect-sha256 <digest> --json`, explains both the
  exact-observation takeover and mechanically derived action, and requires fresh
  explicit confirmation before repeating those operands with `--apply`. It routes
  `completed_unverified` to verification-only recovery and never repeats complete,
  clears a lock, applies recovery, reuses a failed candidate, or retries work
  automatically.
- When a caller-owned adapter, provider CLI, or operator reports quota
  exhaustion, the skill labels that condition and any stated reset as attributed
  external evidence rather than Taskrail attestation. It keeps supervising the
  one running coordinator so already-launched siblings drain and ordinary
  integration, delivery, and unpublished-candidate results settle; it does not
  interrupt the coordinator, replace a worker, launch another frontier, skip an
  aggregate gate, or treat free-form output as authority over Taskrail's terminal
  result.
- The skill preserves available coordinator and worker streams only in a safe
  caller-owned destination outside committed state, warns that provider output
  may be sensitive and incomplete, and reports preservation failure explicitly.
  It quotes reset information with its source and supplied timezone/offset, never
  invents a reset instant, refunds or carries forward the consumed attempt budget,
  or conflates agent execution with delivery-only or transaction recovery.
- Post-reset work is offered only as a new invocation with a fresh result
  destination, exact dry-run and preflight, current retained/unpublished-work
  explanation, newly explicit finite iteration/parallel budget, and fresh operator
  confirmation. The skill never sleeps in the background, persists launch intent,
  resumes a worker/session or `in_progress` outcome, or automatically relaunches.
- The skill concludes with separate worker, integration, delivery, local-gate,
  remote-CI, result-file, and recovery statuses. It claims success only when the
  operator's confirmed delivery and CI policy are satisfied, and labels any
  caller-supplied CI observation as external evidence rather than Taskrail
  attestation.
- The embedded source, installed copies, committed parity mirrors, skill catalog,
  command/workflow guidance, and changelog remain provider-neutral and explicitly
  distinguish the parent agent's supervision from the coordinator's authority.
  The shipped skill contains no source-checkout temporary-loop or
  `--resume-delivery` contract.

## Verification Notes

- Agent Skills format and package-parity tests cover the new skill in embedded,
  installed, and committed forms.
- Behavioral fixtures cover already-specified versus elicited choices, safe
  defaults, invalid/missing child argv, sequential and parallel dry-run summaries,
  confirmation refusal, exact one-shot argv/result destination, worker completion
  order differing from integration rank, clean and conflicting replay, aggregate
  failure, partial publication, and proof that the skill performs no Git or
  planning integration mutation itself.
- CI fixtures cover local aggregate pass with remote `not_checked`, explicit
  caller-owned read-only CI observation, review checks pending-to-pass,
  pending-to-fail, unknown, timeout, adapter refusal, and no false green claim.
- Recovery fixtures cover absent/present/malformed result files, retained worker
  and integration workspaces, transaction preview, declined/approved apply,
  takeover-required exact lock identity, same-host live and changed/mixed-byte
  refusal, cross-host attribution, completed-unverified verify-only guidance, and
  assertions that no retry, lock clear, takeover, recovery apply, or temporary-loop
  bundle handling occurs without its exact authorization.
- Quota fixtures cover attributed sequential failure; parallel failure with
  completion order differing from rank and an independently successful sibling;
  ordinary integration and unpublished candidates; absent, malformed, relative,
  timezone-free, and already-past reset text; stream-preservation failure; exact
  consumed-attempt reporting; and fresh dry-run, budget, result destination, and
  confirmation before a later invocation. Negative assertions prove no provider
  parser, authoritative quota outcome, interruption, retry, budget refund,
  background wait, or automatic relaunch.

## Implementation Notes
