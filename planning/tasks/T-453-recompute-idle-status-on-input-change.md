---
id: T-453-recompute-idle-status-on-input-change
title: Recompute idle status in repair, spec activate, and repoint
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-452-anchor-idle-blocked-status
updated_at: "2026-11-25T04:11:00Z"
---

# T-453-recompute-idle-status-on-input-change Recompute idle status in repair, spec activate, and repoint

## Description

Follow-up derived from T-452-anchor-idle-blocked-status's discovery.

After T-452, when `current_task` is empty, the idle `status_summary` and
`next_action` are a pure projection of three inputs: the `blockers` ledger, the
task files' statuses and `spec_ref`s, and the active spec. Today only
`complete`, `block`, `unblock`, and `task release` recompute that projection.
Commands that change an input, and the command that repairs stale state, leave
it stale:

- `spec activate` changes the active spec but keeps the old summary. This is how
  a downstream repository read `blocked` on four off-spec blockers while `next`
  selected eligible active-spec work.
- `task repoint` can move a blocked task onto or off the active spec.
- `repair` deliberately skips the idle/blocked direction (QC-1, T-052 audit)
  because it assumes two safe values exist. That no longer holds once the value
  is a function of the ledger, task files, and active spec, and `repair --apply`
  is the sanctioned way to heal `STATE.md` written by an older binary.

Outcome: every writer that changes an input of the idle projection, plus
`repair`, recomputes it through the shared `reconcileIdlePointers`. After any
command, `STATE.md` never reads `blocked` because of blockers away from the
active spec. `next` stays out of scope: it is a selection probe whose
documented writes are `next_action`/`updated_at`, and none of its writes change
an input.

This changes documented command contracts (`spec activate` "repoints the active
spec and nothing else"; `repair` leaving idle/blocked alone). The spec text,
`docs/commands.md`, the `taskrail-repair` skill, and doc comments change
together with the behavior.

## Acceptance

- With no active task, `spec activate` recomputes `status_summary` and
  `next_action` against the newly active spec. Leaving only away-from-spec
  blockers yields `idle` and `Select the next eligible task`; an active-spec
  blocker yields `blocked` naming it. With an active task, both fields are
  unchanged. Activation still never changes task files or task statuses.
- With no active task, `task repoint` of a blocked task recomputes both fields:
  repointing the only active-spec blocker away yields `idle`; repointing an away
  blocker onto the active spec yields `blocked` naming it.
- `repair` (dry run) reports `status_summary` and `next_action` changes when the
  idle projection is stale, and `repair --apply` writes them. A `STATE.md` that
  reads `blocked` with only away-from-spec blockers is repaired to `idle`. Repair
  still writes only `STATE.md`, still refuses when more than one task is
  `in_progress`, and keeps the existing single-`in_progress` direction.
- `next` behavior and its written fields are unchanged.
- The spec text for these commands, `docs/commands.md`, the `taskrail-repair`
  skill (package and committed copies via `task skills:regen`), the affected doc
  comments, and `CHANGELOG.md` describe the new behavior.

## Verification Notes

- Failing-first unit tests per writer: `spec activate` with only away blockers,
  with an active blocker, and the active-task no-op; `task repoint` in both
  directions; `repair` dry run and apply on a stale `blocked` state, plus the
  multiple-`in_progress` refusal.
- Regression: a `next` test proves `status_summary` is untouched by `next`.
- Manual sandbox: reproduce the downstream case with a pre-fix `STATE.md`
  (`blocked`, only off-spec blockers), run `repair` then `repair --apply`, and
  confirm `idle`; then `spec activate` back to the blocker's spec and confirm
  `blocked`.
- Full gate: `gofmt -l .`, `go vet ./...`, `go test ./...`, `task check:skills`,
  `taskrail validate`.

## Implementation Notes
