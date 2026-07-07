---
name: autonomous-recovery
description: Recover deterministic tracked-work state when Taskrail planning files are inconsistent
---

# autonomous-recovery

Recover deterministic tracked-work state when Taskrail planning files are
inconsistent, routing every correction through the CLI — never by hand-editing
authoritative state.

## Required Flow

1. Run `go run ./cmd/taskrail validate --json` and inspect violations.
2. Run `go run ./cmd/taskrail repair` to preview the conservative, mechanical
   corrections (a stale `current_task` pointer, stale task counts). This is a dry
   run: review the proposed changes and body diff before applying.
3. Run `go run ./cmd/taskrail repair --apply` to write the reconciled STATE.md and
   re-run validation.
4. Run `go run ./cmd/taskrail validate --json` again.
5. If violations remain, they are outside repair's mechanical scope (a missing
   `spec_ref`, a dependency cycle, more than one in_progress task). Resolve them
   through the tracked-work commands, never by editing STATE.md or task status
   fields by hand.

## Rules

- never hand-edit `planning/STATE.md` frontmatter or task status fields; route
  repairs through `taskrail repair`
- never force progress by casual state mutation
- repair the underlying inconsistency instead of hiding it
- do not implement unrelated product changes during recovery-only runs
