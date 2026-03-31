# AGENTS.md
Guidance for coding agents working in the Taskrail repository.

## Scope And Intent

- This is a Go CLI repository: `github.com/tessariq/taskrail`.
- Main executable: `./cmd/taskrail`.
- Internal packages: `./internal/...`.
- Product specs live under `./specs/`.
- Planning and tracked work live under `./planning/`.
- Keep changes small, explicit, and easy to inspect.
- Until Taskrail is fully usable, this repository dogfoods an adapted workflow using `planning/`, `docs/workflow/`, and mirrored skills.

## Source-Of-Truth Files

- Product specifications: `specs/v0.1.0.md`, `specs/v0.2.0.md`, `specs/v0.3.0.md`
- Spec reading order and versioning: `specs/README.md`
- Active planning state: `planning/STATE.md`
- Tracked tasks: `planning/tasks/`
- Workflow contract: `docs/workflow/`
- Build and convenience commands: `Taskfile.yml`
- CI checks: `.github/workflows/ci.yml`
- Canonical skills: `skills/`
- Mirrored runtime skills: `.agents/skills/` and `.claude/skills/`

## Toolchain And Environment

- Go version: `1.26` (`go.mod`).
- `task` is optional convenience. Direct Go commands remain canonical.
- Prefer repository-local, inspectable file operations over hidden automation.

## Build, Format, And Test Commands

### Build

- Build CLI: `go build ./cmd/taskrail`
- Task wrapper: `task build`

### Formatting And Static Checks

- Check formatting: `gofmt -l .`
- Apply formatting: `gofmt -w .`
- Vet: `go vet ./...`

### Tests

- Run all tests: `go test ./...`
- Run one package: `go test ./internal/taskrail`
- Run one test: `go test ./internal/taskrail -run '^TestValidateState$'`
- Task wrapper: `task test`

### CLI Smoke Checks

- Root help: `go run ./cmd/taskrail --help`
- Validate current repo: `go run ./cmd/taskrail validate`

### Workflow Checks

- Validate Taskrail structure: `go run ./cmd/taskrail validate`
- Select next task: `go run ./cmd/taskrail next --json`
- Check skill mirrors: `./scripts/check-skill-mirrors.sh`

## Repository Workflow Rules

- `planning/STATE.md` is the authoritative execution state.
- Tasks under `planning/tasks/` must declare spec references and dependencies.
- Use `taskrail start`, `complete`, `block`, and `verify` for tracked status transitions once the CLI exists.
- Do not hand-edit task statuses or machine-managed state fields unless the task is explicitly about Taskrail bootstrapping before the CLI is available.
- Verification artifacts belong under `planning/artifacts/verify/`.
- Follow-up work discovered during verification should become new task files.

## Coding Style Guidelines

- Always run `gofmt` on changed Go files.
- Prefer focused functions and explicit data structs.
- Prefer the standard library first.
- Keep the markdown contract easy for humans and agents to inspect.
- Avoid hidden state and avoid over-abstracting simple file operations.

## Testing Expectations

- Follow TDD for code changes whenever practical.
- Keep unit tests dominant.
- Use temporary directories for filesystem-level tests.
- Do not introduce Testcontainers here; Taskrail is a repo-local CLI and should keep test infrastructure light.
- Add smoke coverage for CLI wiring and focused behavior coverage for task parsing, validation, selection, transitions, and verification artifacts.
- Run manual testing against task acceptance criteria for user-visible CLI changes, tracked-work transitions, verification/reporting behavior, and non-trivial workflow changes.
- Store manual test artifacts under `planning/artifacts/manual-test/<task-id>/<timestamp>/`.
- Prefer sandbox-mode manual testing first: temp directories, local CLI execution, and small helper programs when needed.
- Use ephemeral `manual_test` Go-tag tests only when a real CLI flow is hard to validate otherwise, and delete them after writing the report.

## Change Checklist For Agents

- Run `gofmt -w` on edited Go files.
- Run `go vet ./...` for non-trivial changes.
- Run targeted tests for touched packages.
- Run `go test ./...` before handing off substantial changes.
- Run manual testing and persist `plan.md` and `report.md` artifacts for changes that alter Taskrail's visible workflow behavior.
- Run `go run ./cmd/taskrail validate` when changing planning files, task schema, state schema, or spec references.
- Run `./scripts/check-skill-mirrors.sh` when changing canonical skills or mirrored skill directories.
- Update `README.md` when CLI commands or workflow expectations change.
- Update `CHANGELOG.md` for user-visible behavior changes.
- Delete all ephemeral manual test code after the report is written; never commit `*_manual_test.go` files or `cmd/manual-test-*/` directories.

## Do And Don't

- Do keep changes focused on one logical outcome.
- Do reference concrete evidence paths in verification notes.
- Do preserve the distinction between Taskrail product behavior and temporary bootstrap scaffolding.
- Don't add built-in LLM-provider integration in `v0.1.0`.
- Don't turn Taskrail into a sandbox/runtime manager.
- Don't add sandbox, runtime, or container orchestration semantics to this project.
