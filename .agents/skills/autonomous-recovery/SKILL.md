---
name: autonomous-recovery
description: Recover deterministic tracked-work state when Taskrail planning files are inconsistent
---

# autonomous-recovery

Recover deterministic tracked-work state when Taskrail planning files are inconsistent.

## Required Flow

1. Run `go run ./cmd/taskrail validate --json` and inspect violations.
2. If state or task metadata is inconsistent, identify the minimal safe repair.
3. Prefer repairing through Taskrail commands when possible.
4. If bootstrap-era manual edits are required, keep them explicit and limited to restoring valid state.
5. Run `go run ./cmd/taskrail validate --json` again.

## Rules

- never force progress by casual state mutation
- repair the underlying inconsistency instead of hiding it
- do not implement unrelated product changes during recovery-only runs
