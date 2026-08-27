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

1. Run `${TASKRAIL:-taskrail} validate --json`.
2. Choose the task to verify and read its acceptance criteria with
   `${TASKRAIL:-taskrail} task show <task-id> --json`. Consume the returned
   content; do not open the logical task path directly.
3. Run `${TASKRAIL:-taskrail} status --json` and
   consume `storage.artifacts_dir`.
4. Immediately before verification, apply the source-checkout guard when it
   applies. Run `${TASKRAIL:-taskrail} verify <task-id> --result pass --summary "..." --json`, or
   use `${TASKRAIL:-taskrail} verify <task-id> --result fail --summary "..." --json` when findings remain.
5. Confirm returned `artifact_dir`, `plan_path`, and `report_path` are beneath
   the exact transient root reported by `status`.
6. Review unresolved findings.
7. Create a follow-up task with `${TASKRAIL:-taskrail} task new --follow-up <task-id> --title "..." --json`
   (or `${TASKRAIL:-taskrail} verify <task-id> --result fail --summary "..." --create-followup --json`)
   when unresolved work should enter the backlog.

## Rules

- verification-only runs should not mutate unrelated product code
- keep concrete local artifact paths in ephemeral reports; committed task notes
  and `STATE.md` use portable summaries
- treat `planning/STATE.md` as current state, never as a task/session log; do not
  append verification handoff prose to it
- keep verification grounded in the active spec and the task acceptance criteria
- create follow-up tasks with `${TASKRAIL:-taskrail} task new`, never by hand-authoring markdown
