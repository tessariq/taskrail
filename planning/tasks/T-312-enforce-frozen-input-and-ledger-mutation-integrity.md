---
id: T-312-enforce-frozen-input-and-ledger-mutation-integrity
title: Enforce frozen input and ledger mutation integrity
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-172-enforce-autonomous-loop-lifecycle-and-delivery
updated_at: "2026-08-08T14:23:09Z"
---

# T-312-enforce-frozen-input-and-ledger-mutation-integrity Enforce frozen input and ledger mutation integrity

## Description

Compare post-child repository state with the frozen pre-child read set and enforce
the exact task-ledger mutation allowlist for every lifecycle candidate. Produce
ordered integrity violations without deciding committed/local Git delivery.

## Acceptance

- Postflight rechecks active spec, config/layout, storage mode/root, configured and
  effective review policy, prompt template/rendering, staged executable, selected
  and complete task bytes, verification artifacts/IDs, attached full ref, local
  `refs/*`, uppercase root-ref candidates, index/status, and lock identity against
  the frozen snapshot wherever the later delivery contract does not explicitly
  permit a change.
- Pre-existing non-selected tasks remain byte-identical. The selected task may
  change only canonical lifecycle fields/timestamps, blocker, and Implementation
  Notes; ID/title/priority/spec_ref/dependencies, body criteria, and task-local
  `loop_policy`/`loop_reason` remain unchanged.
- New live tasks are allowed only when each is proven by the selected task's fresh
  valid `verify --create-followup` report chain, depends on the selected task,
  omits both loop-policy fields, and remains implicitly held. Any other creation,
  deletion, rename, explicit allowance, or ledger mutation is integrity failure.
- Complete local refs remain byte-identical except for the attached branch advance
  evaluated by T-313. Every captured uppercase root-ref candidate remains
  byte-identical and no new matching regular entry appears, excluding only
  `COMMIT_EDITMSG`; aliases, special files, and namespace changes fail closed.
- Violations use exact stable code/message/nullable-path objects ordered by code,
  null-last path, then message. Final equality is claimed only for snapshots that
  can prove it; transient ref/reflog movement and opaque prompt behavior are not
  fabricated as mechanical evidence.

## Verification Notes

- Mutation-matrix fixtures perturb every frozen control input and each selected,
  non-selected, new, deleted, renamed, and policy-field ledger boundary, asserting
  exact ordered violations and unaffected allowed lifecycle changes.
- Follow-up fixtures cover zero, one, and many report-proven implicit-hold tasks,
  plus wrong dependency, absent report, stale chain, explicit policy, and unrelated
  task creation.
- Git namespace fixtures cover tags, notes, remote-tracking and sibling refs,
  attached-branch allowance handoff, captured/new uppercase root candidates,
  aliases, special files, `EVIL_REV`, and the documented transient-movement limit.

## Implementation Notes
