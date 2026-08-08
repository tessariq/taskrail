# Human Workflow

How a human developer should work when Taskrail tracked-work state exists.

## Normal Flow

1. Run `go run ./cmd/taskrail validate`.
2. Claim a tracked item with `go run ./cmd/taskrail start <task-id>`.
3. Implement in a TDD loop.
4. Run the appropriate tests.
5. Run manual testing against the task's acceptance criteria when the change alters Taskrail's visible behavior.
6. Write `plan.md` and `report.md` under `planning/artifacts/manual-test/<task-id>/<timestamp>/`.
7. Create any known follow-up task before finishing the lifecycle transition.
8. On success, run `go run ./cmd/taskrail complete <task-id> --note "..."`, then `go run ./cmd/taskrail verify <task-id> --result pass --summary "..."`.
9. If work cannot proceed, run `block --reason` before `verify --result fail`; deliberate rework may record fail while remaining `in_progress`.

## What Does Not Change

- branch strategy
- commit strategy
- pull request workflow
- implementation ownership and design judgment

Taskrail adds deterministic state handling. It does not replace engineering judgment.

Repository-wide human context may live in `planning/NOTES.md`. It is human-owned,
not generated state: agents may read it, but edit it only when a human explicitly
asks. Task-specific evidence and history still belong with the task, blocker,
verification report, or follow-up.

## Taskrail Source Checkout

The `go run ./cmd/taskrail ...` commands above always build the current source.
Before using `${TASKRAIL:-taskrail}` instead for `next`, `start`, `verify`,
`complete`, `block`, or any other state writer in Taskrail's own source checkout,
run `task taskrail:check`. It checks the exact binary the command will invoke and
must pass before any tracked bytes change. If it fails, stop and apply the named
build or resolution remedy first. Installed adopter repositories do not carry
Taskrail's source-only freshness helper and need no such check.

## Manual Testing Notes

- Prefer a temporary directory or temporary repository sandbox first.
- Use ephemeral `manual_test` Go-tag tests only when shell-driven validation is not enough.
- Delete all temporary manual test code after writing the report.
- Manual-test and verify artifacts are ephemeral and gitignored; never commit
  them or the temporary test harness.

## Recovery

If state appears inconsistent:

1. run `go run ./cmd/taskrail validate --json`
2. inspect the reported violations
3. repair the repo through normal Taskrail commands — `taskrail repair --apply`
   for mechanical `STATE.md` drift, the ordinary transition commands otherwise

Never mutate `planning/STATE.md` or task statuses by hand. Earlier guidance
allowed "explicit bootstrap edits" because the CLI did not exist yet; it has
shipped since v0.1.0 and owns every tracked-work write. If no command can
express the repair you need, that is a missing capability worth a task, not a
reason to edit state directly.

### Refreshing rendered `STATE.md` after hand-edits

Rendered `STATE.md` body fields (task counts, `current_task`) are projections of
the task files, re-rendered only by state-writing commands. Prefer `taskrail
task new` to author tasks — it refreshes the counts as it creates the file, so
they never drift. If you add, remove, or edit task files by hand (for example
bulk backlog authoring), the rendered counts go stale; run `taskrail repair`
(dry run) then `taskrail repair --apply` to re-project `STATE.md` from the task
files. Repair only ever rewrites `STATE.md` — never a task file — so it cannot
advance a status or fabricate work. There is deliberately no separate "refresh"
command: `repair` already owns re-projecting `STATE.md`.

### Moving an open task onto another spec area

After `spec activate`, an open task whose `spec_ref` still points at the previous
spec is off-spec: `next` skips it and `status` lists it. Use `taskrail task
repoint <id> --area <anchor>` to move it onto the active spec (or `--spec-ref
<path>#<anchor>` for the cross-spec case) rather than hand-editing the
frontmatter. It rewrites only `spec_ref`, re-projects `STATE.md`, and re-runs
`validate`; `--dry-run` previews the change. It never touches the id, filename,
title, status, or dependencies, and completed and cancelled tasks are rejected as
delivered history. Because it writes `STATE.md`, run `git status` afterwards and
stage the regenerated file with the change.
