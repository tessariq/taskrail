---
name: autonomous-verify
description: Run deterministic verification against Taskrail tracked-work acceptance criteria and spec alignment
---

# autonomous-verify

Run deterministic verification against Taskrail tracked-work acceptance criteria and spec alignment.

Requires the installed `taskrail` binary on `PATH`.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Required Flow

1. Run `${TASKRAIL:-taskrail} validate`.
2. Choose the task to verify.
3. Run `${TASKRAIL:-taskrail} verify <task-id> --result pass|fail --summary "..."`.
4. Confirm plan and report artifacts were written under
   `planning/artifacts/verify/`.
5. Review unresolved findings.
6. Create a follow-up task with `${TASKRAIL:-taskrail} task new` (or
   `${TASKRAIL:-taskrail} verify <task-id> --create-followup`) when unresolved work should
   enter the backlog.

## Rules

- verification-only runs should not mutate unrelated product code
- keep concrete local artifact paths in ephemeral reports; committed task notes
  and `STATE.md` use portable summaries
- treat `planning/STATE.md` as current state, never as a task/session log; do not
  append verification handoff prose to it
- keep verification grounded in the active spec and the task acceptance criteria
- create follow-up tasks with `${TASKRAIL:-taskrail} task new`, never by hand-authoring markdown
