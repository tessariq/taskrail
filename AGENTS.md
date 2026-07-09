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
- Release checklist: `docs/workflow/releasing.md`
- Build and convenience commands: `Taskfile.yml`
- CI checks: `.github/workflows/ci.yml`
- Local git hooks: `lefthook.yml` (opt-in pre-commit/commit-msg/pre-push; mirrors CI, but `.github/workflows/ci.yml` is authoritative)
- Canonical skills: `skills/`
- Mirrored runtime skills: `.agents/skills/` and `.claude/skills/`
- Skills productization contract: `docs/workflow/skills-productization.md`

## Toolchain And Environment

- Go version: `1.26` (`go.mod`).
- `task` is optional convenience. Direct Go commands remain canonical.
- `mise` (`mise.toml`) provisions the pinned toolchain (Go, `task`, `lefthook`):
  `mise install` sets it up on a fresh clone and `mise run setup` additionally
  builds the working-tree `taskrail` onto the mise PATH (`./bin`, via
  `task taskrail:install`) and wires the opt-in git hooks (`lefthook install`).
  A bare `taskrail` then resolves to the current build with no `TASKRAIL`
  override; `task taskrail:check` fails loud if that on-PATH binary is stale.
  Locally it is optional
  convenience — direct `go` commands and `Taskfile.yml` targets work without it —
  but CI provisions the same toolchain via `jdx/mise-action`, so `mise.toml` is
  the single source of toolchain versions locally and on CI. The pins are guarded
  by `internal/toolchain`: `go` matches `go.mod`, `lefthook` matches the
  `task hooks:install` guidance, and CI is asserted to provision via mise-action.
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

### Git Hooks (Optional)

- Install once: `task hooks:install` (runs `lefthook install`).
- `lefthook.yml` wires local hooks that mirror CI: `pre-commit` runs `gofmt`, `go vet ./...`, `taskrail validate`, and the skill-mirror check; `commit-msg` enforces Conventional Commits and rejects agent-attribution trailers; `pre-push` runs `go test ./...`.
- Hooks are an opt-in convenience. CI remains the authoritative gate. Do not bypass with `--no-verify`.

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
- Keep functions focused with early returns; aim well under ~50 lines.
- Keep files 200–400 lines as a norm, 800 as a hard ceiling; extract when a file does too much. (`internal/taskrail/service.go` at ~640 lines is acceptable but on the larger side and a candidate to split.)
- Avoid nesting deeper than ~3–4 levels; prefer early returns over deep branches.
- Comments (and godoc) explain *why*, not *what*: do not restate the code or a signature; reserve them for non-obvious rationale, invariants, and contracts.
- Prefer the standard library first.
- Keep the markdown contract easy for humans and agents to inspect.
- Avoid hidden state and avoid over-abstracting simple file operations.

## Commit Conventions

- Use Conventional Commits: `<type>: <description>` (types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`).
- Reference the task ID as a parenthetical suffix on the subject, never as a prefix: `feat: add version reporting to taskrail CLI (T-012)`, not `feat: T-012 add version reporting`.
- Keep the subject imperative and scoped to one logical outcome; use the body to explain the why when it is not obvious.

### Committing Tracked-Work State

- One commit per tracked task: include the implementation, its tests, and the workflow metadata the CLI regenerated (`planning/STATE.md`, rewritten task files, `CHANGELOG.md`) in the **same** commit. The regenerated state is part of the task's logical outcome — do not split it into a separate follow-up/chore commit.
- Whenever a change alters tracked-work or spec state that `STATE.md` reflects — task status transitions, added/removed tasks, task counts, active spec/version, or a verification result — stage the CLI-regenerated `planning/STATE.md` (and any task files the CLI rewrote) in that commit so committed state stays consistent.
- `STATE.md` is generated by the CLI (`start`, `next`, `verify`, `complete`, `block` all rewrite it). Never hand-edit it, but always commit the version the CLI produced. **Run `git status` after any tracked-work command** to catch regenerated files before committing.
- Create follow-up tasks before the task's final transition (or via `taskrail verify --create-followup`) so `STATE.md` counts include them; a task file added after the last state-writing command will not appear in committed `STATE.md` until the next one runs.
- Follow-up task files created while completing a task belong in **that task's commit**, next to its `STATE.md` update — they are part of the same tracked-work outcome, not a separate change. Because `STATE.md` is one cumulative snapshot, a follow-up's `STATE.md` count and its task file must land together.
- If asked to split the work into multiple commits, never separate `planning/STATE.md` or a task's added/rewritten task files from the functionality that produced them. `STATE.md` is a single cumulative snapshot, so splitting it from its task files produces a counted-but-absent (or absent-but-counted) inconsistency. Surface this, keep each task's implementation + tests + `STATE.md` + task files in one commit, and split only along genuinely independent concerns.

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
- After any `taskrail start`/`next`/`verify`/`complete`/`block`, run `git status` and stage the regenerated `planning/STATE.md` and rewritten task files with the related change; never leave committed `STATE.md` out of sync with task/spec state.
- Update `README.md` when CLI commands or workflow expectations change.
- Update `CHANGELOG.md` for user-visible behavior changes under an Unreleased
  section; skip internal-only refactors, CI plumbing, and dependency-bump noise.
  Keep entries terse: **one to two lines**, lead with the command or user-facing
  verb, state the observable effect and the flags a user types. Leave out internal
  mechanics (function names, struct/schema ids, `embed.FS`, "shared validator")
  and design rationale — those belong in the commit body or spec. Fold one
  user-facing feature into one entry even when it spans several tasks. Copy-edit
  against the existing entries so register and length stay consistent; the terse
  v0.1.0 entries are the reference.
  - Good: `` `taskrail repair` — reconcile mechanical `STATE.md` drift; dry run by
    default, `--apply` writes `STATE.md` only and re-validates. Supports `--json`. ``
  - Bad: a 5-sentence paragraph restating the task description and how it works
    internally.
- Delete all ephemeral manual test code after the report is written; never commit `*_manual_test.go` files or `cmd/manual-test-*/` directories.

## Notes On Repository Behavior

Intentional, non-obvious decisions — do not "fix" these:

- `validate` is read-only. It never writes `planning/STATE.md` or task files.
- `start`, `next`, `verify`, `complete`, and `block` rewrite `planning/STATE.md` (and sometimes task files). Even a `next` selection probe updates `next_action`/`updated_at`, so it dirties the working tree — check `git status` after running it.
- Rendered `STATE.md` counts are a projection of the task files. `taskrail task new` refreshes them as it creates a task (prefer it over hand-authoring). If you hand-add/remove/edit task files, refresh the projection with `taskrail repair --apply` — there is no separate "refresh" command; `repair` owns re-projecting `STATE.md` and never touches task files or status.
- `verify` creates `planning/artifacts/verify/<id>/<timestamp>/` on demand; the artifacts tree is gitignored and is never committed (no `.gitkeep` placeholders — the v0.2.0 gitignored-artifacts contract).
- Committed `STATE.md` stays portable: `last_verification_result` is a path-free summary and `relevant_artifacts` is empty, so cloned repos never point at producer-only files.
- Manual-test artifacts under `planning/artifacts/manual-test/` are ephemeral local evidence and are gitignored.
- Git hooks (`lefthook.yml`) are an opt-in local mirror of CI. If lefthook is not installed they simply do not run, and CI still gates — never rely on hooks as the only check, and never bypass them with `--no-verify`.

## Boundaries

**Always:**

- Run `gofmt`, `go vet ./...`, and targeted tests before handing off.
- Start behavior changes with a failing test.
- Route tracked-work transitions through the CLI (`start`, `complete`, `block`, `verify`).
- Commit the CLI-regenerated `planning/STATE.md` (and rewritten task files) with the change that produced it.
- Keep each change focused on one logical outcome.
- Reference concrete evidence paths in verification notes.
- Preserve the distinction between Taskrail product behavior and temporary bootstrap scaffolding.

**Ask first:**

- Changing the task or state schema, spec contracts, CI checks, or the skill-mirror structure.
- Adding a runtime dependency (prefer the standard library).
- Broad refactors beyond the task's scope.

**Never:**

- Hand-edit `planning/STATE.md` or task status fields once the CLI exists.
- Commit anything under `planning/artifacts/`, or add `.gitkeep` placeholders.
- Add built-in LLM-provider integration in `v0.1.0`.
- Turn Taskrail into a sandbox/runtime manager or add container-orchestration semantics.
- Bypass hooks or CI-equivalent checks with `--no-verify`.
