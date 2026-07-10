---
name: autonomous-recovery
description: Recover deterministic tracked-work state when Taskrail planning files are inconsistent
---

# autonomous-recovery

Recover deterministic tracked-work state when Taskrail planning files are
inconsistent, routing every correction through the CLI — never by hand-editing
authoritative state.

Requires the installed `taskrail` binary on `PATH`. Run it from the managed
repository's root.

## Required Flow

1. **Inspect.** Run `${TASKRAIL:-taskrail} validate --json` and read the
   violations.
2. **Dry-run.** Run `${TASKRAIL:-taskrail} repair` to preview the conservative,
   mechanical corrections (a stale `current_task` pointer, a `status_summary`
   stale against a single `in_progress` task, stale rendered task counts). This
   defaults to a dry run: review the proposed frontmatter changes and the
   `STATE.md` body diff before applying.
3. **Apply.** Run `${TASKRAIL:-taskrail} repair --apply` to write the reconciled
   `STATE.md` and re-run validation.
4. **Re-validate.** Run `${TASKRAIL:-taskrail} validate --json` again.
5. If violations remain, they are outside repair's mechanical scope (a missing
   `spec_ref`, a dependency cycle, more than one `in_progress` task). Resolve them
   through the tracked-work commands, never by editing `STATE.md` or task status
   fields by hand.

## Rules

- never hand-edit `planning/STATE.md` frontmatter or task status fields; route
  every correction through `${TASKRAIL:-taskrail} repair`
- never force progress by casual state mutation
- repair the underlying inconsistency instead of hiding it
- do not implement unrelated product changes during recovery-only runs
