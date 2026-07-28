# Changelog

All notable user-visible changes to Taskrail will be documented in this file.

## Unreleased

### Added

- `taskrail spec diff <from> <to>` — read-only, mechanical anchor-set delta between
  two specs: added areas (need decomposition), removed areas (orphan existing
  tasks), and best-effort rename candidates. Reuses the `spec show --anchors` slug
  logic; never writes `STATE.md` or task files and never gates `validate`. Supports
  `--json`.
- `taskrail next --include-off-spec` — one-shot recovery that ranks eligible
  `todo` tasks across all specs and flags an off-spec pick (`off-spec:` marker,
  `off_spec:true`, and a `selected_off_spec` warning in `--json`). Default `next`
  stays active-spec-filtered; writes no more state than a normal selection probe.

- `taskrail init --with-skills` — installed skill files now record the Taskrail
  version that wrote them as a `taskrail_version` frontmatter key, so a skill
  installed by an older binary is detectable. `--force` restamps; re-running the
  same version stays a no-op and writes no backups.
- Every command now warns on stderr when the installed skills were written by a
  different Taskrail version, naming the affected skills, both versions, and
  `taskrail init --with-skills --force`. Skills with no marker report once as
  unknown-version and prescribe no remedy. Advisory only: `--json` stdout stays
  parseable, `validate` still passes, and no transition is blocked.
- `taskrail status` — active-spec drift breakdown: counts open work
  (`todo`/`in_progress`/`blocked`) on the active spec versus away from it, and
  lists the away tasks with their `spec_ref`. The away set matches the
  active-spec filter `next` uses for idle selection. Read-only; `--json` mirrors
  the counts and task/spec-ref pairs.
- `taskrail coverage --gaps` — advisory read-only structural gap analysis over
  covered active-spec areas: `missing-verification`, `dependency-anomaly`, and
  `under-decomposed-area` candidates to promote into tasks. Composes with
  `--area` to scope the report to one coverable area; the narrowed report names
  the selected area (even when it has no gaps). Advisory by default;
  `--fail-on <category>` opts into an exit-code
  CI gate that reds the build when a signal of a named category is present
  (repeatable or comma-separated; report unchanged, never affects `validate`).
  Supports `--json`.
- `taskrail-decompose` skill — `init --with-skills` now also installs it; drafts
  spec-anchored tasks for uncovered active-spec areas by composing `coverage
  --json`, `spec show --anchors`, and `import --apply` (draft-only; no new command).
- `taskrail-gap` skill — `init --with-skills` now also installs it; pairs
  `coverage --gaps --json` structural candidates with agent semantic gap review
  over covered active-spec areas, proposing tasks a human promotes via `task new`
  / `import --apply` (advisory-only; no new command).
- `taskrail task rename <id>` — atomically re-slug a task: rewrite its `id`,
  rename the file (`git mv` when tracked), and fix every inbound `dependencies:`
  reference. `--slug` sets the slug; `--title` derives it. `--dry-run` previews
  the change set; `--json` emits it. Preserves the `T-<n>` prefix; never advances
  status. A selector that normalizes to no slug de-slugs the task back to the bare
  `T-<n>` id and warns on stderr. Also repoints the body's `# <id> <title>`
  heading, reported as a `body_heading` change.
- `taskrail task new --area <anchor>` — active-spec shorthand for `--spec-ref
  <active-spec-path>#<anchor>`. Mutually exclusive with `--spec-ref`; an unknown
  anchor fails before writing and points at `spec show <active-version> --anchors`.
- `taskrail task repoint <id>` — re-point an open task's `spec_ref` onto a new
  area without hand-editing frontmatter. `--area <anchor>` resolves it against the
  active spec, `--spec-ref <path#anchor>` sets it explicitly (mutually exclusive);
  an unknown anchor fails before writing. Rewrites only `spec_ref`, then
  re-projects `STATE.md` — check `git status` afterwards. Completed and cancelled
  tasks are rejected. `--dry-run` previews the change and the validity it would
  produce, writing nothing; `--json` emits the change.

### Changed

- `taskrail next` — anchor idle selection to the active spec: only `todo` tasks
  whose `spec_ref` points at the active spec are considered, so older-spec work is
  skipped, not selected. When only older-spec work is runnable, `next` reports no
  eligible task and lists it under `warnings` (`skipped_non_active_spec`). An
  already-active task pointing outside the active spec is still returned with a
  `selected_non_active_spec` warning. `status` mirrors the same read-only selection.
- `taskrail task new` — derive a slugged id and filename from `--title`
  (`T-<n>-<slug>`); `--slug` overrides the derived slug. With neither flag the id
  stays the bare `T-<n>`. Accented letters transliterate to ASCII before slugifying
  (`Über Fußball` → `ueber-fussball`, `Łódź Điện` → `lodz-dien`, in precomposed or
  decomposed input alike). A title-derived slug is length-capped (~50 chars, on a
  hyphen boundary); an explicit `--slug` is written verbatim. A `--title`/`--slug`
  that normalizes to no slug keeps the bare id but warns on stderr.

### Fixed

- Every command now refuses a repository recording a `layout_version` newer than
  the binary supports ("upgrade taskrail"), before reading or writing state —
  previously only `init` checked.
- `taskrail complete`, `block`, and `unblock` now reject a `--note`/`--reason`
  that embeds a gitignored `planning/artifacts/` file path before writing,
  pointing you at a path-free summary — previously the transition wrote committed
  state that `validate` then failed. `verify --create-followup` applies the same
  guard to the follow-up task's title/description (`--summary`/`--details`).
- `taskrail verify` and `taskrail block` no longer append a second
  `## Implementation Notes` heading when writing the first note to a task.

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
