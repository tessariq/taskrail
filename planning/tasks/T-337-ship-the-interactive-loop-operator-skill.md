---
id: T-337-ship-the-interactive-loop-operator-skill
title: Ship the interactive loop operator skill
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-244-publish-streamed-loop-results-out-of-band
    - T-335-deliver-parallel-batches-through-review-adapters
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
  inspects `taskrail lock status --json`, previews an identified transaction with
  `taskrail recover <transaction-id> --json`, explains the mechanically derived
  action, and requires fresh explicit confirmation before `--apply`. It routes
  `completed_unverified` to verification-only recovery and never repeats complete,
  clears a lock, applies recovery, reuses a failed candidate, or retries work
  automatically.
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
  lock-held and mixed-byte refusal, completed-unverified verify-only guidance, and
  assertions that no retry, lock clear, recovery apply, or temporary-loop bundle
  handling occurs without its exact authorization.

## Implementation Notes
