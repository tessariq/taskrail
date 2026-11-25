---
id: T-452-anchor-idle-blocked-status
title: Anchor idle blocked status to active-spec blockers
status: completed
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies: []
updated_at: "2026-11-25T04:13:02Z"
completion_id: "bc82d4e5192e95a87a723b5196e36d59"
last_verification_id: "7dbbe7de3b6ca53f58d88b755de5118b"
last_verification_result: pass
last_verified_at: "2026-11-25T04:13:02Z"
last_verified_completion_id: "bc82d4e5192e95a87a723b5196e36d59"
---

# T-452-anchor-idle-blocked-status Anchor idle blocked status to active-spec blockers

## Description

With no active task, the `STATE.md` `status_summary` and the `Resolve blocker
on <id>` next action are set from the blockers ledger. They currently treat
every ledger entry as blocking, so a repository whose only blockers point away
from the active spec reads `blocked` while `next` selects eligible active-spec
work. Example: a downstream v0.4.0 repository with active spec v0.3.4 had four
off-spec blockers.

Outcome: the idle summary is anchored to the active spec, as `next` selection
is (`specs/v0.5.0.md#canonical-transition-order` reprojection, and the v0.4.0
active-spec drift breakdown). Only blockers on active-spec tasks make the state
`blocked` and become the `next_action` target. Away-from-spec blockers keep their
ledger entries and reasons, and human `taskrail status` marks them as away from
the active spec so they do not read as active-spec blockers.

## Acceptance

- After `complete`, `block`, `unblock`, or `task release` leaves no active task,
  `status_summary` is `idle` and `next_action` is `Select the next eligible task`
  when every remaining blocker's task points away from the active spec.
- When an active-spec blocker and a later-recorded away-from-spec blocker both
  remain, `status_summary` is `blocked` and `next_action` names the active-spec
  blocker.
- Away-from-spec blockers remain in the `STATE.md` `blockers` ledger with their
  reasons.
- A ledger entry whose task file is absent still counts as blocking.
- Human `taskrail status` suffixes away-from-spec blocked lines with
  `(away from active spec)` and leaves active-spec blocked lines unmarked;
  `status --json` is unchanged.

## Verification Notes

- Unit: `go test ./internal/taskrail -run '^TestIdleSummaryIgnoresBlockersAwayFromActiveSpec$'`
  covers the idle, mixed-order, and ledger-retention criteria.
- CLI smoke: `go test ./cmd/taskrail -run '^TestStatusMarksBlockersAwayFromActiveSpec$'`
  covers the human status marker.
- Manual sandbox: two specs, block an old-spec task, activate the new spec, then
  `status` shows the away marker, and completing an active-spec task re-projects
  `## Status` to `idle` while the blocker stays listed.
- Full gate: `gofmt -l .`, `go vet ./...`, `go test ./...`, `taskrail validate`.

## Implementation Notes

- 2026-11-25T04:12:39Z: Idle status_summary and next_action are now derived only from active-spec blockers; away-from-spec blockers stay in the ledger and human status marks them (away from active spec). Failing-first tests added for both.
- 2026-11-25T04:13:02Z: verification pass id 7dbbe7de3b6ca53f58d88b755de5118b previous none completion bc82d4e5192e95a87a723b5196e36d59
