# Changelog

All notable user-visible changes to Taskrail will be documented in this file.

## Unreleased

### Added

- `taskrail spec` — spec command family. `spec activate <version>` repoints the
  active spec in `STATE.md` to `specs/<version>.md`, re-renders `STATE.md`, and
  re-validates; it is the CLI-only writer of the active spec and rejects a missing
  or non-conforming version with no write. Supports `--json`.
- `taskrail coverage` — advisory read-only linkage analysis for the active spec:
  a decomposition-coverage percentage with per-area gaps, orphan tasks (spec_ref
  pointing at another spec), and a two-directional drift summary. Never writes
  state and never fails `validate`; a spec with no coverable areas reports `N/A`.
  Supports `--json`.
- `taskrail status` — strictly read-only snapshot of current tracked-work state:
  active spec, task counts (done/active/blocked/todo), the next eligible task
  marked *not persisted*, blocked tasks with reasons, the last verification
  result, and a one-line coverage summary (`N/A` when the spec has no coverable
  areas). Leaves the working tree clean. Supports `--json`.
- `taskrail stats` — strictly read-only aggregate statistics computed
  snapshot-only from current task files and `STATE.md`: counts and percentages by
  status, the blocked ratio and recorded-blocker count, spec coverage with a
  per-area breakdown, and dependency shape (tasks with unmet dependencies,
  longest dependency chain). Leaves the working tree clean. Supports `--json`.

### Changed

- Shipped agent skills now invoke the CLI through `${TASKRAIL:-taskrail}` instead
  of a hardcoded `taskrail`. Adopters need nothing (it resolves to the installed
  binary); set `TASKRAIL=/path/to/taskrail` (or `go run ./cmd/taskrail`) to override.

### Fixed

- `taskrail block` now keeps every currently-blocked task's reason in `STATE.md`
  (one entry per task) instead of overwriting the list with only the most recent;
  `taskrail status` reports each blocked task's own reason.

## v0.2.0 - 2026-07-07

Second release. Taskrail builds on the stable v0.1.0 repo contract to make adoption
in existing repositories easy: guided retrofit, LLM-free import of rough notes into
spec/task drafts, opt-in shippable agent skills, a version-aware non-destructive
`init`, and conservative mechanical `STATE.md` repair — all while keeping the core
CLI provider- and tooling-independent.

### Added

- `taskrail repair` — reconcile mechanical `STATE.md` drift (stale `current_task`
  pointer or task counts) against the task files. Dry run by default; `--apply`
  rewrites `STATE.md` only (never a task file) and re-validates. Judgement calls
  (missing `spec_ref`, dependency cycles, multiple in_progress) are left to
  `validate`. Supports `--json`.
- `taskrail task new` — scaffold a task file with the next free id. Requires
  `--title` and `--spec-ref`; supports `--priority`, repeatable `--dep`, `--json`.
  Runs `validate`'s checks at creation so an invalid task never lands.
- `taskrail task new --follow-up <parent-id>` — scaffold a follow-up: inherits the
  parent's `spec_ref` and adds it as a dependency.
- `taskrail import` — build spec/task drafts from a markdown source without an LLM.
  `--to tasks|spec|planning` previews a draft; `--emit-prompt` prints a paste-ready
  agent prompt; `--apply <draft.json>` validates and writes real files. Supports
  `--json`. (`--llm` deferred to v0.3.)
- `taskrail retrofit [notes]` — guided bootstrap for a non-standard repo: detect
  layout, scaffold, and adopt reviewed notes as tracked work. Dry run by default;
  `--apply` scaffolds without overwriting. Supports `--json`.
- `taskrail init --with-skills` — install the shippable tracked-work agent skills
  (`autonomous-backlog`, `autonomous-task`, `autonomous-verify`, `taskrail-repair`,
  `taskrail-import`, `taskrail-retrofit`). Opt-in; re-running never overwrites edits.
- `taskrail init` is now version-aware and non-destructive: writes a
  `.taskrail/config.yml` layout marker, adopts an existing v0.1.0 layout, and
  migrates older layouts (dry run, `--apply` to write). Never rewrites human content.
- `taskrail validate` now detects dependency cycles and committed references to
  gitignored `planning/artifacts/` paths.
- Homebrew install: `brew install tessariq/tap/taskrail` (macOS and Linux).

### Changed

- `taskrail import --apply` is now atomic — pre-flights all checks before writing,
  so a failing draft leaves the repo unchanged.
- `taskrail verify` records a portable, path-free result in committed `STATE.md`;
  gitignored artifact paths no longer leak into `relevant_artifacts`.
- `taskrail init` no longer pre-creates gitignored artifact directories; `verify`
  creates them on demand.

### Fixed

- `taskrail validate` no longer fails on a fresh clone when the gitignored
  `planning/artifacts` tree is absent.

## v0.1.0 - 2026-06-19

First shippable release. Taskrail is a manual-first, LLM-provider-agnostic CLI for
repo-native tracked work, proving the repository contract, deterministic task
progression, the authoritative `STATE.md`, and verification as a first-class concept.

### Added

- `taskrail init` — initialize Taskrail structure (`specs/`, `planning/`, starter `STATE.md`) in the current repository.
- `taskrail validate` — validate folder layout, task shape, dependency and spec references, and `STATE.md` consistency.
- `taskrail next` — deterministically select the next eligible task (supports `--json`).
- `taskrail start <task-id>` — mark a task active and update `STATE.md`.
- `taskrail complete <task-id>` — mark a task completed from an implementation perspective (supports `--note`).
- `taskrail block <task-id>` — mark a task blocked and record a `--reason`.
- `taskrail verify <task-id>` — record a verification outcome and write artifacts under `planning/artifacts/verify/`; can create a follow-up task via `--create-followup`.
- `taskrail version` — print the CLI version (also `--version`), injected at build time via `-ldflags`.
- Bootstrap repository structure, specs, planning workflow, and mirrored skills.
