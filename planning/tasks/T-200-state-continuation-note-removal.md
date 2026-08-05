---
id: T-200-state-continuation-note-removal
title: Bound STATE continuity and plan continuation-note removal
status: completed
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies: []
updated_at: "2026-08-05T18:31:11Z"
---

# T-200-state-continuation-note-removal Bound STATE continuity and plan continuation-note removal

## Description

Stop treating `STATE.md` continuation notes as an implicit task/session log. Make
fresh layout-1 repositories start without bootstrap prose, define the final
schema removal as part of the transactional layout-2 upgrade, and steer agents
to task notes, verification summaries, blockers, and follow-up tasks instead.

## Acceptance

- Fresh `taskrail init` and `retrofit --apply` state carries an empty
  `continuation_notes` list; existing layout-1 repositories and their authored
  notes remain byte-preserved until an ordinary writer re-renders them.
- The v0.5 layout-upgrade contract removes `continuation_notes` and the rendered
  `## Notes` section in state schema v2. Migration preview exposes existing
  notes, and apply requires `--drop-continuation-notes` when any are non-empty;
  no migration silently discards authored text.
- The migration-only flag remains supported while any direct or multi-hop
  upgrade from state schema v1 is supported, is reported as unnecessary when no
  notes need removal, and is removed only when v1 migration support is dropped.
- T-157 owns the transactional schema-v2 migration and its rollback, old-binary,
  strict-decoding, fresh-init, and acknowledgement coverage; T-185 carries the
  same acknowledgement through supported multi-hop `1 -> 2 -> 3` upgrades.
- README/workflow guidance and every relevant packaged skill say that
  `STATE.md` is CLI-managed current state, never a per-task/session log. They
  direct durable task context to task `## Implementation Notes`, blockers,
  verification summaries/reports, or follow-up tasks.
- The autonomous verification skill's generic “notes” wording names task/report
  notes explicitly and cannot be read as permission to edit `STATE.md`.
- Packaged and committed skill copies stay byte-identical.
- Tests cover the empty fresh-state seed and preserve existing continuation
  notes under current schema-v1 writers.
- README and CHANGELOG describe the observable fresh-init and agent-workflow
  behavior without claiming schema v2 has shipped.
- `gofmt -l .`, `go vet ./...`, `go test ./...`, skill parity, task-body hygiene,
  and `taskrail validate` pass.

## Verification Notes

- In a temporary repository, initialize with and without packaged skills,
  inspect `STATE.md`, and confirm no bootstrap continuation prose is seeded.
- Seed a layout-1 repository with an authored continuation note, run an existing
  state writer, and confirm the note is preserved rather than silently removed.

## Implementation Notes

- 2026-08-05T18:31:03Z: verification pass
- 2026-08-05T18:31:11Z: Stopped fresh continuation-note seeding, clarified packaged agent guidance, and assigned schema-v2 removal plus explicit acknowledgement to T-157/T-185; automated and manual verification pass
