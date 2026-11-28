# AGENTS.md

Guidance for coding agents in the Taskrail repository, a Go CLI
(`github.com/tessariq/taskrail`). This file is the single agent-instruction
source and must work in every agent tool. Rules belong here; checks that must
always run belong in `lefthook.yml` and CI. Do not add tool-specific harness
config such as `.claude/settings.json`, `.claude/rules/`, or content in
`CLAUDE.md`, which only imports this file.

Before editing Go code, read `internal/AGENTS.md` (it applies to `cmd/` too).
Before editing `internal/taskrail/` or the packaged skills, read
`internal/taskrail/AGENTS.md`.

## Product Boundaries

- Taskrail is released software. This repository dogfoods its own CLI,
  `planning/`, `docs/workflow/`, and packaged skills exactly like an adopter.
- Never add built-in LLM-provider integration or direct model calls to Taskrail
  core (`specs/v0.5.0.md#recommendation-about-llm-support`). `loop` and skill
  evaluation stay provider-neutral.
- Never turn Taskrail into a sandbox or runtime manager, and add no
  container-orchestration semantics.
- Product scope comes from the specs. Reading order: `specs/README.md`. The
  active spec is `active_spec_version` in `planning/STATE.md`.

## Setup

- Run `mise run setup` before your first commit in a worktree; do not assume the
  git hooks are installed. It provisions the pinned toolchain (`mise.toml`),
  builds the working-tree `taskrail` onto PATH, and installs the lefthook hooks.
- Commit under the maintainer's git identity: set `user.name` and `user.email`
  to a person before your first commit. `scripts/check-author.sh` refuses agent
  identities in pre-commit, pre-push, and CI.

## Definition Of Done

| Change | Run before handing off |
|---|---|
| Go code | `task check` (mirrors the CI `checks` and `build-test` jobs) |
| `planning/`, `specs/`, docs, `AGENTS.md` | `task check:planning` (mirrors the planning CI lane) |
| Packaged skills | `task skills:regen`, then `task check` |

- While iterating, run one package or test: `go test ./internal/taskrail -run '^TestValidateState$'`.
- A new active-spec area needs a linked task, or `task coverage:gate`
  (`coverage --min 100`) fails even though `validate` passes.
- Tests pin the wording of `AGENTS.md`, `README.md`, and `docs/`, and full CI
  skips doc-only changes. `task check:docs` runs those tests; the pre-commit
  hook and the planning lane run it too.
- For user-visible CLI or workflow changes, also run sandbox manual tests and
  keep `plan.md` and `report.md` under
  `planning/artifacts/manual-test/<task-id>/<timestamp>/`
  ([procedure](docs/workflow/development-workflow.md#manual-testing)). Never
  commit `*_manual_test.go` files or `cmd/manual-test-*/` directories.

## Running The CLI

- In this checkout, prefer `go run ./cmd/taskrail <command>`: it always builds
  the current source.
- If you run bare `taskrail` (the packaged skills use `${TASKRAIL:-taskrail}`),
  run `task taskrail:check` immediately before every state writer and apply the
  remedy it prints. A binary older than the working tree writes old behavior
  into task files without erroring (T-123; background in
  `docs/binary-resolution-findings.md`).
- Inspect `taskrail <command> --help` rather than guessing flags.

## Tracked Work

- `planning/STATE.md` is a CLI-generated projection of the task files, not a
  task or session log. Never hand-edit `planning/STATE.md`, task status fields,
  or other machine-managed fields; `task check:state` (pre-commit and CI) fails when
  `STATE.md` differs from its projection.
- Re-project mechanical drift, including a merge or rebase conflict on
  `STATE.md`, with `taskrail repair --apply`; never resolve its conflict markers
  by hand (skill `taskrail-repair`).
- Route every transition through the CLI: `taskrail start`, `complete`,
  `block`, `unblock`, and `verify`. If no command expresses the change you need,
  that is a missing capability: file a task instead of hand-editing, even when
  the active spec describes a surface that is not implemented yet.
- The [Command effects](docs/commands.md#command-effects) table is the list of
  commands that write tracked files. After any writer, run `git status`, run
  `taskrail validate`, and commit the regenerated `STATE.md` and task files with
  the change that produced them
  ([procedure](docs/workflow/development-workflow.md#committing-tracked-work-state)).
- Create tasks with `taskrail task new` and move one onto another spec area
  with `taskrail task repoint`. Tasks declare spec references and dependencies.
- Before `start`, confirm the task is one independently meaningful outcome with
  a bounded implementation, review, and verification surface. Never split by
  file, layer, discipline, phase, or estimate. Re-plan oversized or ambiguous
  work first; running out of context or review budget never turns unfinished
  scope into a follow-up
  ([task readiness](docs/workflow/agent-steering.md#task-readiness)).
- Follow-up work found during verification becomes new task files, created
  before the task's final transition or with `verify --create-followup`.
- `planning/NOTES.md` is human-owned: read it, and edit it only when a human
  explicitly asks.
- Verification and manual-test artifacts live under `planning/artifacts/`,
  which is gitignored. Never commit them or add `.gitkeep` placeholders.

## Review

- For ordinary spec and task-file edits, run the deterministic checks once after
  the change settles; do not launch semantic review agents unless asked.
- A review request authorizes one bounded review wave over one frozen snapshot.
  Batch findings and fixes, and do not start another wave because files changed
  or to gain confidence
  ([review cadence](docs/workflow/agent-steering.md#review-cadence)).
- Unshipped active-spec review workflows describe future product behavior, not
  contributor checks, unless a task or maintainer invokes them.

## Notes On Repository Behavior

Intentional decisions; do not "fix" them. Details in `docs/commands.md`.

- Read-only commands (`validate`, `status`, `stats`, `coverage`, `task show`,
  `spec list/show/diff`) never write tracked files. `status` computes the next
  task without persisting it; `next` writes `next_action` and `updated_at`.
- `coverage --gaps` is advisory by default and never writes; `--fail-on
  <category>` opts into an exit code, and gap findings never make `validate` fail.
- `spec diff` reports and never migrates. `spec activate` rewrites only
  `STATE.md`.
- Committed `STATE.md` stays portable: `last_verification_result` is path-free
  and `relevant_artifacts` is empty.

## Commits

- Use Conventional Commits, `<type>: <description>`, with the types `feat`,
  `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`. Keep the subject
  imperative and scoped to one logical outcome.
- Suffix the short task key when there is one: `feat: add version reporting
  (T-012)`, never the slugged identifier and never a prefix.
- Every ordinary commit needs a body that explains intent, context, and
  non-obvious decisions, wrapped at 72 characters.
- Never add agent attribution: no `Co-authored-by:`, `Assisted-by:`, or agent
  session or thread trailers, no bare session link, and no robot mark.
  `scripts/check-attribution.sh` is the one policy that `commit-msg`,
  `pre-push`, and CI apply.
- Make one commit per tracked task: implementation, tests, and the
  CLI-regenerated `STATE.md`, task files, and follow-up tasks together. Never
  split `STATE.md` from the task files it counts.
- Update `CHANGELOG.md` under Unreleased for user-visible changes
  ([policy](docs/workflow/changelog.md)). When a command changes, update
  `docs/commands.md` (including Command effects) and `README.md`.
- Never bypass hooks with `--no-verify`; CI re-checks the commit policy for
  every pushed range.

## Ask First

- Changing the task or state schema, spec contracts, CI checks, or the skill
  package-parity structure.
- Adding a runtime or developer-tool dependency.
- Broad refactors beyond the task's scope.
- Pushing, tagging, or releasing ([checklist](docs/workflow/releasing.md)).
