---
name: taskrail-repair
description: Conservatively repair mechanical Taskrail STATE.md drift through taskrail repair, never by hand-editing state
---

# taskrail-repair

Reconcile `planning/STATE.md` with the task files when they have drifted
mechanically — a `current_task` pointer that disagrees with the in_progress task, a
`status_summary` stale against a single in_progress task, or stale rendered task
counts — without ever hand-editing authoritative state. The
`taskrail` binary proposes and applies only conservative, mechanical corrections;
it never advances a task status or invents work.

Requires the installed `taskrail` binary on `PATH`. Run it from the managed
repository's root.

## Flow

1. **Inspect.** Run `${TASKRAIL:-taskrail} validate --json` and read the violations. Repair
   only heals mechanical STATE.md drift; other violations (a missing `spec_ref`, a
   dependency cycle, more than one in_progress task) need human judgement.
2. **Dry-run.** Run `${TASKRAIL:-taskrail} repair`. This defaults to a dry run: it prints the
   proposed frontmatter corrections and the STATE.md body diff and writes nothing.
3. **Review.** Confirm every proposed change follows the task files (the source of
   truth) and only touches STATE.md. If a change would advance a status or
   fabricate work, stop — repair does not do that, so an unexpected proposal means
   the drift is not mechanical and needs manual investigation.
4. **Apply.** Run `${TASKRAIL:-taskrail} repair --apply` to write STATE.md and re-run
   validation.
5. **Re-validate.** Run `${TASKRAIL:-taskrail} validate` and confirm the state is valid. Any
   violation that remains was outside repair's mechanical scope; resolve it
   deliberately, never by editing STATE.md by hand.

## Rules

- never hand-edit `planning/STATE.md` frontmatter or task status fields; route
  every correction through `${TASKRAIL:-taskrail} repair`
- dry-run and review before `--apply`
- repair only reconciles mechanical drift; it never advances status or invents work
- inconsistencies repair leaves untouched are surfaced by validation — fix them
  through the tracked-work commands, not by editing state
