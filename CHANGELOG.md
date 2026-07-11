# Changelog

All notable user-visible changes to Taskrail will be documented in this file.

## Unreleased

### Added

- Windows install via WinGet: `winget install Tessariq.Taskrail` (amd64/arm64),
  published from the tagged release. The GitHub Release also gains Windows `.zip`
  assets. Availability follows a moderated PR to `microsoft/winget-pkgs`.
- `taskrail unblock <task-id>` — return a blocked task to todo so it re-enters
  `next` selection and drop its `STATE.md` blocker entry (other blocked tasks keep
  theirs); optional `--reason` appends a note. Rejects a non-blocked task with no
  write. Supports `--json`.
- `taskrail spec` — spec command family. `spec activate <version>` repoints the
  active spec in `STATE.md` to `specs/<version>.md`, re-renders `STATE.md`,
  re-validates, and prints the one-line coverage summary plus any tasks still
  pointing at the previously active spec (informational — activation succeeds
  regardless); it is the CLI-only writer of
  the active spec and rejects a missing or non-conforming version with no write.
  `spec list` lists the versioned specs
  and marks the active one; `spec show <version>` prints a spec, or with
  `--anchors` its `spec_ref` heading anchors exactly as `validate` accepts them.
  `spec add <version>` scaffolds `specs/<version>.md` with the standard section
  skeleton and adds it to the `specs/README.md` reading order without activating
  it. `list` and `show` are read-only. Supports `--json`. Shell completion
  (`taskrail completion <shell>`) completes spec versions for `spec show`/`spec
  activate` and real `<path>#<anchor>` values for `task new --spec-ref`.
- `taskrail coverage` — advisory read-only linkage analysis for the active spec:
  two coverage figures over the same areas — decomposition (any linked task) and
  report-only implementation (every linked task completed) — with per-area state
  (uncovered / decomposed / implemented), a reverse map listing the covering
  task id(s) for each area (double-covered areas flagged), orphan tasks (spec_ref
  pointing at another spec), and a two-directional drift summary. `status` and
  `stats` show both figures. Never writes state and never fails `validate`; a
  spec with no coverable areas reports `N/A`. `--min <pct>` (0–100) opts into CI
  gating: exits non-zero when decomposition coverage is below the threshold
  (report unchanged, `validate` still advisory, `N/A` never gates). Supports
  `--json`.
- `taskrail status` — strictly read-only snapshot of current tracked-work state:
  active spec, task counts (done/active/blocked/todo), the next eligible task
  marked *not persisted*, blocked tasks with reasons, the last verification
  result, a one-line coverage summary (`N/A` when the spec has no coverable
  areas), and a one-line orphan/drift summary (orphan-task and uncovered-area
  counts) alongside it. Leaves the working tree clean. Supports `--json`.
- `taskrail stats` — strictly read-only aggregate statistics computed
  snapshot-only from current task files and `STATE.md`: counts and percentages by
  status, the blocked ratio and recorded-blocker count, spec coverage with a
  per-area breakdown, and dependency shape (tasks with unmet dependencies,
  longest dependency chain). Leaves the working tree clean. Supports `--json`.
  `--format dot|mermaid` instead exports the task dependency DAG (nodes = tasks,
  edges = dependencies) as Graphviz DOT or Mermaid text for external rendering.

### Changed

- CLI file-read errors now name a repo-relative path (e.g. `read spec file
  specs/v9.9.9.md: no such file or directory`) instead of leaking the absolute
  repository location.
- Shipped agent skills now invoke the CLI through `${TASKRAIL:-taskrail}` instead
  of a hardcoded `taskrail`. Adopters need nothing (it resolves to the installed
  binary); set `TASKRAIL=/path/to/taskrail` (or `go run ./cmd/taskrail`) to override.
- `taskrail init --with-skills` now also installs the `autonomous-recovery`,
  `autonomous-manual-test`, and `taskrail-spec` agent skills; still opt-in and
  non-destructive.
- `taskrail init --with-skills --force` reinstalls the embedded skills over
  existing copies for upgrades, backing up any locally-modified file to a
  timestamped sibling first and reporting the overwritten and backed-up paths.
  Without `--force`, behavior is unchanged (already-installed skills are left in
  place).
- `taskrail repair` also reconciles a `status_summary` that is stale against a
  single `in_progress` task (sets it to `in_progress`); still `STATE.md`-only,
  dry run by default, and never advances a status. The idle/blocked direction and
  multiple `in_progress` tasks stay human-resolved.

### Fixed

- `taskrail task new` now allocates the next id from the highest numeric prefix
  across bare and slug-suffixed ids, so a repo whose ids are all slug-suffixed
  (e.g. `T-076-ingestion-commands`) no longer restarts at `T-001` and collides.
  `taskrail validate` now reports two task files that share a numeric prefix
  (e.g. `T-001` and `T-001-milestone`) as a collision.
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
