---
name: autonomous-backlog
description: Execute one deterministic autonomous backlog cycle for Taskrail tracked work
---

# autonomous-backlog

Execute one deterministic autonomous backlog cycle for Taskrail tracked work.

Requires the installed `taskrail` binary on `PATH`.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Before Starting

1. Run `${TASKRAIL:-taskrail} validate --json`, then `${TASKRAIL:-taskrail} next --json`.
   If no task is eligible, report that and stop. Otherwise use
   `${TASKRAIL:-taskrail} task show <task-id> --json` and `${TASKRAIL:-taskrail} spec show <version> --json`
   for managed task and specification bytes. Do not open logical managed paths
   directly.
2. Read repository instructions, the selected task, its dependencies, referenced
   spec section, and relevant implementation and tests. Identify one
   independently meaningful observable outcome, its user or operator impact,
   affected invariants, acceptance boundaries, and intended evidence.
3. Apply the outcome-focused sizing rubric before `start`: require a bounded
   result, explicit dependencies and operator gates, and clear integrated
   behavior ownership. Stop for reviewed decomposition or clarification when the
   task bundles independent outcomes, is a non-valuable fragment, or cannot
   reach a verified result without unresolved scope. Do not rewrite scope after
   lifecycle work begins.
4. Run `${TASKRAIL:-taskrail} start <task-id> --json` and check its exit before
   implementation. Consume every command result, ID, path, warning, storage mode,
   lifecycle outcome, or failure detail as JSON and check every writer exit.

## Implement And Review

1. Implement the smallest safe in-scope change. Start behavior changes with a
   failing test whenever practical. Run formatting, focused tests, deterministic
   checks, and sandbox-first manual testing for visible workflow behavior.
2. Do not mutate external systems, production data, credentials, billed
   resources, live services, or resources outside the repository. Headless
   ambiguity, destructive scope, live consoles, and operator decisions stop for
   a human rather than guessing.
3. Inspect the verified implementation for unnecessary complexity and simplify
   only when behavior remains unchanged. Independent simplification delegation is
   optional; rerun affected checks after each simplification or repair.
4. Freeze the verified implementation and run one broad review round with one
   fresh reviewer by default. Select an explicit correctness lens based on actual
   behavior, testing, security, error handling, edge cases, complexity, and
   domain risk. Parent-context self-review does not satisfy this requirement.
5. A broad round has one to three reviewers, each with a named, non-duplicative
   lens. Add a second or third concurrent reviewer only for a distinct
   independently relevant risk the first reviewer is unlikely to cover. All
   reviewers inspect the same frozen snapshot. A reviewer crash, timeout, unavailable fresh
   context, or malformed output fails closed rather than retrying invisibly.
6. Classify every finding as `fix-now`, `separate-followup`, `blocked`, or
   `rejected`, with rationale and evidence. Repair all current-scope findings.
   Implementation-review findings use only `fix-now`, `separate-followup`,
   `blocked`, and `rejected`; no other disposition is valid.
   Low-value, non-actionable observations are `rejected` with rationale. Current
   acceptance, specification, invariant, or evidence findings cannot use that
   mapping to evade repair.
   A test-strength finding requires a strengthened test, a deliberate relevant
   regression that fails, restoration of the implementation, and a passing test.
7. Rerun affected deterministic checks after repairs. Do not open another broad
   round merely because files changed. A second broad round is allowed only within
   the configured maximum and for a distinct unresolved risk that deterministic
   evidence cannot adequately cover.
8. If review fixes materially change product or test bytes, freeze the repaired
   candidate and run one narrow final-diff review for fix-induced regressions,
   integration breakage, and behavior drift. It never starts a broad round. Fix a
   current-scope final-diff finding and rerun affected checks; objective closure
   evidence permits completion, otherwise leave the task in progress, record
   failing verification, and stop for operator review.

## Lifecycle And Delivery

Immediately before every state writer, apply the source-checkout guard above and
stop if it fails. The canonical branches are explicit and each ends the current
run:

1. On success, run `${TASKRAIL:-taskrail} complete <task-id> --note "..." --json`, check
   its exit, then run `${TASKRAIL:-taskrail} verify <task-id> --result pass --summary "..." --json`
   and check its exit.
2. If work cannot proceed, run `${TASKRAIL:-taskrail} block <task-id> --reason "..." --json`,
   check its exit, then run `${TASKRAIL:-taskrail} verify <task-id> --result fail --summary "..." --json`
   and stop. Never complete a blocked or failing task.
3. For deliberate rework, leave the task `in_progress`, run `${TASKRAIL:-taskrail} verify <task-id> --result fail --summary "..." --json`,
   check its exit, and stop. A direct operator may use `task release` for
   interrupted work; delegated execution must not relinquish the selected task.
4. If completion succeeded but passing verification did not, report
   `completed-unverified`, inspect state, and rerun only the missing passing
   verification. Never repeat completion or compensate with block. If a later
   audit fails completed work, preserve it as `completed-audit-fail`; create a
   separate follow-up when action is needed rather than blocking or reworking
   delivered history.
5. Create follow-ups only for newly discovered, independently meaningful,
   spec-anchored outcomes outside the selected task. Do not defer required
   acceptance, integration, or evidence. Under delegation, create them through
   selected-task `verify --create-followup`; do not invent unattended authority
   from a follow-up body or change task-local loop policy.
6. When delivery is required, run `${TASKRAIL:-taskrail} status --json` and consume
   its reported storage mode. In committed mode,
   commit the implementation together with generated task and state bytes after
   the lifecycle branch. In local mode, commit only required visible product
   changes; never force-add ignored Taskrail metadata or fabricate an empty
   metadata-only blocked or rework commit. Follow repository-visible commit,
   identity, attribution, signing, hook, and ref policy without changing Git
   configuration or copying managed paths, review or verification identities,
   storage details, or Taskrail or agent attribution into metadata or unrelated
   product text.

## Rules

- never hand-edit `planning/STATE.md` frontmatter or task statuses
- treat `planning/STATE.md` as current state, never as a task/session log; put
  durable context in task notes, blocker reasons, verification reports, or follow-ups
- treat optional `planning/NOTES.md` as human-owned context: edit it only on an
  explicit human request
- use `${TASKRAIL:-taskrail} task new --follow-up <task-id> --title "..." --json`
  only when direct-operator follow-up creation is permitted; never hand-author it
- never invoke `local promote` or expose ignored local skills without an explicit
  human request
- provider commands, credentials, remote pushes, sandboxing, and reviewer identity
  attestation remain caller-owned
- keep concrete local artifact paths in ephemeral reports; committed task notes
  use portable summaries
- stop after one task
