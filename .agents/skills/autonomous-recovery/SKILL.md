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

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Recovery Procedure

Read and follow [taskrail-repair](../taskrail-repair/SKILL.md), the canonical
mechanical recovery procedure installed alongside this skill. Its inspection,
reviewed dry-run, apply, revalidation, and refusal boundaries apply to this entry
point too. If the sibling skill cannot be loaded, stop and report the missing
procedure; do not improvise a replacement or hand-edit state.

## Rules

- never hand-edit `planning/STATE.md` frontmatter or task status fields; use the
  canonical procedure above
- treat `planning/STATE.md` as current state, never as a task/session log; do not
  add continuation prose while recovering it
- never force progress by casual state mutation
- repair the underlying inconsistency instead of hiding it
- do not implement unrelated product changes during recovery-only runs
