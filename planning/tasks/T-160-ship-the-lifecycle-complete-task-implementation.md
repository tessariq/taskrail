---
id: T-160-ship-the-lifecycle-complete-task-implementation
title: Ship the lifecycle-complete task implementation prompt
status: completed
priority: high
spec_ref: specs/v0.5.0.md#task-implementation-prompt
dependencies:
    - T-217-release-interrupted-active-work-safely
    - T-241-warn-on-passing-verification-before-completion
    - T-297-ship-complete-storage-neutral-prompt-rendering
    - T-251-ship-the-outcome-focused-task-authoring-prompt
    - T-303-align-native-task-producers-with-the-body-contract
updated_at: "2026-08-22T11:08:14Z"
completion_id: "67ed73c0d331169c8475da9f61526f88"
last_verification_id: "f85ba03fa93df1c99a4a09be9236aa3c"
last_verification_result: pass
last_verified_at: "2026-08-22T11:08:14Z"
last_verified_completion_id: "67ed73c0d331169c8475da9f61526f88"
---

# T-160-ship-the-lifecycle-complete-task-implementation Ship the lifecycle-complete task implementation prompt

## Description

Ship the built-in task implementation prompt with bounded independent review,
simplification, finding dispositions, lifecycle/delivery branches, and recovery
guidance. Packaged full-task skill command execution is owned by T-242.

## Acceptance

- Rendering uses only declared path/ID/version placeholders and instructs
  inspection of repository files; it never interpolates body contents or
  undeclared context.
- The workflow covers freshness before every writer, start, implementation,
  deterministic verification, optional behavior-preserving simplification,
  regression perturbation, one focused fresh-context reviewer by default,
  risk-justified additional reviewers, correction of every current-scope finding,
  deterministic re-verification, and a conditional non-recursive final-diff review.
- Every finding receives `fix-now`, `separate-followup`, `blocked`, or `rejected`
  plus rationale. One broad round is the default; a configured second round is
  used only for a distinct unresolved risk, not merely because findings were
  repaired. Material review-induced product or test changes require one narrow
  final-diff review. A final-diff finding is repaired and re-verified; objective
  evidence that directly demonstrates closure permits completion without another
  model review, otherwise the task remains in progress with failing verification.
  The final-diff review never starts another broad round.
- Success completes then passes; cannot-proceed blocks then fails; deliberate
  rework may remain in progress with fail. Each branch checks writer exits, then
  follows the selected storage-mode delivery contract: committed mode creates one
  commit containing implementation and generated task/state bytes; local success
  commits product bytes only, while metadata-only local blocked/rework may retain
  HEAD and never fabricates an empty commit. Completed-unverified/audit recovery
  reruns only the safe step before any required commit.
- Every consumed command uses the common JSON result, and interrupted/manual
  rework guidance names direct-operator `task release` without allowing a
  delegated child to relinquish its selected task.
- Headless ambiguity, credentials, destructive scope, and barriers stop for a
  human; necessary independently meaningful out-of-scope follow-ups are created
  only through selected-task verification, with no arbitrary numeric cap, no
  `loop_policy`/`loop_reason`, and no current-task repair deferred into follow-up.
- Delegated task implementation may use only its granted lifecycle and follow-up
  write sets. It cannot allow, hold, clear, or otherwise mutate task-local loop
  policy, and authoring a follow-up body cannot grant unattended authorization.
- The prompt leaves provider command, credentials, remote push, and sandboxing to
  callers and never claims reviewer identity attestation.
- Prompt guidance inventories existing repository primitives, stops on material
  ambiguity, and traces requirements to observable executable evidence before
  implementation.
- Before `start`, the workflow applies the T-251 semantic sizing rubric and stops
  for reviewed replanning when the selected task bundles independent outcomes,
  fragments one outcome, or leaves integration ownership unclear. It does not
  rewrite scope after lifecycle work begins.
- Follow-ups are limited to newly discovered, independently meaningful out-of-scope
  outcomes; they cannot carry fragments required to complete, integrate, or verify
  the selected outcome.
- Committed mode delivers implementation plus generated planning bytes; local
  mode commits visible product changes only and leaves a valid ignored Taskrail
  lifecycle/verification outcome.
- Local delivery follows repository-visible commit, identity, attribution,
  signing, hook, and ref policy; it neither changes Git identity configuration nor
  copies ignored Taskrail IDs, managed paths, review/verification provenance,
  storage details, or invented Taskrail/agent attribution into commit metadata or
  unrelated product text. Only caller-owned instruction outside managed planning
  can authorize exposing a local Taskrail identity/path in commit metadata.
  Frozen repository-visible policy governs generic Git conventions, but
  planning-derived policy cannot launder that authority across runs.
  Outcome-required product bytes may contain a Taskrail reference.

## Verification Notes

- Map each branch to golden/mutation fixtures proving lifecycle-before-commit,
  generated-byte inclusion, source guard, simplification consideration without
  mandatory delegation, one default reviewer, risk-justified additional lenses
  and second-round use, deterministic post-fix checks, final-diff objective
  closure and unresolved-judgment rework, perturbation, barriers, implicitly held
  follow-ups, delegated policy refusal, exits, recovery, local provenance
  minimization, repository-policy exceptions, and unchanged Git
  identity/configuration.
- Manually render path-valued context and exercise success, blocked, rework, and
  partial-completion instructions without provider invocation by Taskrail.
- Exercise oversized, fragmented, and unclear-integration selections to prove the
  pre-start stop/replan branch, plus in-scope and independently meaningful
  out-of-scope discoveries to prove follow-up routing.

## Implementation Notes

- 2026-08-22T11:08:03Z: Published the lifecycle-complete task implementation prompt with contract coverage.
- 2026-08-22T11:08:14Z: verification pass id f85ba03fa93df1c99a4a09be9236aa3c previous none completion 67ed73c0d331169c8475da9f61526f88
