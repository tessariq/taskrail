# Changelog

All notable user-visible changes to Taskrail will be documented in this file.

## v0.3.0 - 2026-07-14

Third release. Taskrail gains read-only insight into tracked work — `status`,
`stats`, and `coverage` report progress, aggregate metrics, and spec-linkage
without touching state — plus a `spec` command family for inspecting and
authoring specs, `unblock` to release blocked tasks, and Windows install via
WinGet. The core CLI stays provider- and tooling-independent.

### Added

- `taskrail spec` — spec command family. `spec activate <version>` repoints the
  active spec in `STATE.md` and re-validates (the CLI-only writer of the active
  spec); `spec list` and `spec show <version>` (with `--anchors` for `spec_ref`
  values) inspect specs read-only; `spec add <version>` scaffolds a new spec.
  Completion completes spec versions and `<path>#<anchor>` values. Supports `--json`.
- `taskrail coverage` — advisory read-only spec-linkage analysis: per-area
  decomposition and implementation coverage, a reverse map of the covering task
  id(s), orphan tasks, and a drift summary. `--min <pct>` opts into CI gating;
  `--area <anchor>` narrows to one area. Never writes state or fails `validate`.
  Supports `--json`.
- `taskrail status` — read-only snapshot: active spec, task counts, the next
  eligible task (marked not persisted), blockers, last verification, and a
  coverage/drift summary. Leaves the working tree clean. Supports `--json`.
- `taskrail stats` — read-only aggregate metrics: status distribution, blocked
  ratio, spec coverage, and dependency shape. `--format dot|mermaid` exports the
  task dependency DAG instead. Leaves the working tree clean. Supports `--json`.
- `taskrail unblock <task-id>` — return a blocked task to todo so it re-enters
  `next` selection and drop its `STATE.md` blocker entry (others keep theirs);
  `--reason` appends a note. Supports `--json`.
- Windows install via WinGet: `winget install Tessariq.Taskrail` (amd64/arm64),
  with Windows `.zip` assets on the GitHub Release. Availability follows a
  moderated PR to `microsoft/winget-pkgs`.

### Changed

- `taskrail init --with-skills` now also installs the `autonomous-recovery`,
  `autonomous-manual-test`, and `taskrail-spec` skills; `--force` reinstalls the
  embedded skills over existing copies, backing up any locally-modified file
  first. Still opt-in and non-destructive by default.
- Shipped agent skills now invoke the CLI through `${TASKRAIL:-taskrail}`; set
  `TASKRAIL=/path/to/taskrail` to override (it resolves to the installed binary
  otherwise).
- `taskrail repair` also reconciles a `status_summary` left stale against a single
  `in_progress` task; still `STATE.md`-only and dry run by default.
- CLI file-read errors now name a repo-relative path instead of the absolute
  repository location.

### Fixed

- `taskrail task new` now allocates the next id from the highest numeric prefix
  across bare and slug-suffixed ids, so all-slug-suffixed repos no longer restart
  at `T-001` and collide; `validate` now flags two files sharing a numeric prefix.
- `taskrail block` now keeps every blocked task's reason in `STATE.md` instead of
  overwriting the list with only the most recent.
- `taskrail complete` now leaves `status_summary` as `blocked` when other tasks
  remain blocked, instead of resetting to `idle`.

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
