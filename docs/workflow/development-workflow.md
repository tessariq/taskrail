# Development Workflow

Contributor and coding-agent workflow for Taskrail tracked work.

## Source Of Truth

- `Taskfile.yml`
- `.github/workflows/ci.yml`
- `docs/workflow/`
- `planning/STATE.md`
- `planning/tasks/`

## Build And Validation

- Build: `go build ./cmd/taskrail`
- Tests: `go test ./...`
- Workflow validation: `go run ./cmd/taskrail validate`
- CLI smoke: `go run ./cmd/taskrail --help`
- Skill mirror check: `./scripts/check-skill-mirrors.sh`

## TDD Default

For any code change:

1. Write the smallest failing test that captures the behavior.
2. Make the test pass with the minimal implementation.
3. Refactor while keeping the suite green.

## Testing Pyramid

- Unit tests are the default layer.
- Filesystem-level integration tests should use temporary repositories.
- Keep CLI smoke tests sparse and high-signal.

Rules:

- Keep tests deterministic.
- Prefer temporary directories over global state.
- Do not introduce Testcontainers here.

## Manual Testing

After automated checks pass, run manual testing against the task's acceptance criteria when the change affects user-visible Taskrail behavior.

Use manual testing for:

- CLI command behavior and help text changes
- tracked-work state transitions
- verification artifact and reporting behavior
- non-trivial workflow or agent-guidance changes

Default mode: sandbox mode.

- Create `planning/artifacts/manual-test/<task-id>/<timestamp>/plan.md`.
- Derive numbered test steps from the task acceptance criteria.
- Run the steps in a temporary directory or temporary repository using normal `go run ./cmd/taskrail ...` flows.
- Write `planning/artifacts/manual-test/<task-id>/<timestamp>/report.md` with pass, fail, or pass-with-fixes verdicts.

Optional mode: ephemeral `manual_test` Go-tag tests.

- Use this only when a real CLI flow is awkward to validate from shell commands alone.
- Name tests `TestManual_<Name>` and guard them with `//go:build manual_test`.
- Delete the `_manual_test.go` files after the report is written.
- Never substitute ordinary automated tests for manual test evidence.

Cleanup rules:

- Delete `cmd/manual-test-*/` helper directories after the report is written.
- Delete all `_manual_test.go` files after the report is written.
- Commit only the artifacts, not the temporary test code.

## Tracked-Work Commands

- `go run ./cmd/taskrail validate`
- `go run ./cmd/taskrail next --json`
- `go run ./cmd/taskrail start <task-id>`
- `go run ./cmd/taskrail complete <task-id> --note "<evidence>"`
- `go run ./cmd/taskrail block <task-id> --reason "<reason>"`
- `go run ./cmd/taskrail verify <task-id> --result pass|fail --summary "<summary>"`

## What To Run By Change Type

- Docs-only changes: review rendered markdown.
- Small localized code changes: `gofmt`, targeted tests.
- Non-trivial logic changes: `gofmt`, `go vet`, targeted tests, then `go test ./...`.
- User-visible CLI or workflow changes: `gofmt`, `go vet`, `go test ./...`, then manual testing with persisted artifacts.
- Planning or spec changes: `go run ./cmd/taskrail validate`.
- Skill changes: `./scripts/check-skill-mirrors.sh`.
