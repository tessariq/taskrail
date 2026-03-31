# Human Workflow

How a human developer should work when Taskrail tracked-work state exists.

## Normal Flow

1. Run `go run ./cmd/taskrail validate`.
2. Claim a tracked item with `go run ./cmd/taskrail start <task-id>`.
3. Implement in a TDD loop.
4. Run the appropriate tests.
5. Run manual testing against the task's acceptance criteria when the change alters Taskrail's visible behavior.
6. Write `plan.md` and `report.md` under `planning/artifacts/manual-test/<task-id>/<timestamp>/`.
7. Run `go run ./cmd/taskrail verify <task-id> --result pass|fail --summary "..."`.
8. Create a follow-up task if verification finds additional needed work.
9. Finish with `complete` or `block`.

## What Does Not Change

- branch strategy
- commit strategy
- pull request workflow
- implementation ownership and design judgment

Taskrail adds deterministic state handling. It does not replace engineering judgment.

## Manual Testing Notes

- Prefer a temporary directory or temporary repository sandbox first.
- Use ephemeral `manual_test` Go-tag tests only when shell-driven validation is not enough.
- Delete all temporary manual test code after writing the report.
- Commit the artifacts, not the temporary test harness.

## Recovery

If state appears inconsistent:

1. run `go run ./cmd/taskrail validate --json`
2. inspect the reported violations
3. repair the repo through normal Taskrail commands or explicit bootstrap edits

Do not mutate `planning/STATE.md` or task statuses casually once the CLI is in normal use.
