# Changelog

All notable user-visible changes to Taskrail will be documented in this file.

## Unreleased

### Added

- `taskrail init` and `retrofit --apply` create `planning/NOTES.md` for
  human-owned context when absent; an existing file is untouched, and an unsafe
  path (symlink, directory, case alias) refuses with `path_blocked`.

### Changed

- `taskrail validate` accepts paired task-local `loop_policy`/`loop_reason`
  metadata, treats an absent pair as implicit hold, rejects malformed pairs and
  legacy `planning/AUTONOMY.tsv`, and preserves explicit policy across task writes.
- Every `--json` command now emits the v0.5 machine envelope — `schema_version`,
  `command`, `warnings`, and one of `result`/`error` — instead of the bare report
  or writer payload, and a failure emits a registered error code instead of
  prose-only stderr. This one-time direct-result break has no legacy switch;
  `schema_version` versions the whole document contract.
- `taskrail start`, `complete`, and `block` gain `--json`, reporting the
  transition and the validation re-run alongside it. Text output is unchanged.
- `taskrail init --json` now reports the layout marker, the write inventory,
  the `planning/NOTES.md` disposition with its continuation-note choices, and —
  with `--with-skills` — every installed skill path, replacing the prose
  `changes` list. `--with-skills` installs are no longer text-only.
- `taskrail status --json` now reports `storage` with the active mode, root, and
  physical `artifacts_dir` for transient staging.
- Warnings are published only in the envelope's `warnings` array; commands no
  longer repeat them inside their result payloads. Text output still shows them
  on stderr, and they never change the exit status.
- `taskrail import --apply` now reports a partial apply as a `partial_write`
  error naming the paths it wrote, replacing the `"partial": true` result.
- Task operands now require the exact full persisted ID, and `taskrail validate`
  rejects broken v0.5 completion and verification metadata chains.
- `taskrail init --with-skills` now installs Agent Skills-compliant copies with
  `metadata.taskrail_version`; `--force` safely normalizes legacy markers.
- `taskrail init` and `retrofit --apply` no longer seed generic continuation
  prose in fresh `STATE.md` files; packaged workflow guidance keeps durable task
  context in task notes, blockers, verification reports, or follow-up tasks.

## v0.4.0 - 2026-07-30

Fourth release. Taskrail makes active-spec work safer to author and
select, adds atomic task rename/repoint operations and mechanical spec/gap review,
and makes binary, layout, and installed-skill version skew visible before it can
damage tracked state. The core remains deterministic and provider-independent.

### Added

- `taskrail spec diff <from> <to>` — read-only, mechanical anchor-set delta between
  specs: definitive added/removed areas plus supplemental best-effort rename
  candidates. Never writes tracked state or gates validation; supports `--json`.
- `taskrail task rename <id>` — atomically re-slug a task's id and filename, body
  heading, inbound dependencies, and current-task pointer. `--slug` is normalized
  but uncapped; `--title` derives a capped slug. Supports `--dry-run` and `--json`.
- `taskrail task repoint <id>` — move an open task's `spec_ref` with `--area` or
  `--spec-ref`, then re-project `STATE.md`. Supports `--dry-run` and `--json`.
- `taskrail task new --area <anchor>` — resolve a task's `spec_ref` against the
  active spec without copying its path; mutually exclusive with `--spec-ref`.
- `taskrail next --include-off-spec` — one-shot recovery that ranks runnable work
  across all specs and clearly flags an off-spec selection in text and JSON.
- `taskrail status` — active-spec drift breakdown: counts open work
  on and away from the active spec, then lists away-task ids and `spec_ref` values.
  Read-only; supports `--json`.
- `taskrail coverage --gaps` — advisory read-only structural gap analysis over
  covered active-spec areas. Scope with `--area`; opt into an exit-code policy with
  `--fail-on`; consume structured candidates with `--json`.
- `taskrail-decompose` and `taskrail-gap` skills — draft tasks for uncovered spec
  areas and add semantic review to mechanical gap signals; proposals remain
  reviewable and require explicit promotion into tracked state.
- `taskrail init --with-skills` — stamp installed skills with the writing Taskrail
  version. Commands warn about stale copies and name `init --with-skills --force`
  as the explicit remedy without blocking transitions or corrupting JSON output.

### Changed

- Packaged state-writing skills now run the source checkout's
  `task taskrail:check` before tracked bytes change, refusing stale or wrongly
  resolved working-tree binaries without affecting installed adopter workflows.
- `taskrail next` — anchor idle selection to the active spec: only `todo` tasks
  on that spec are ranked by default. Older runnable work is reported as skipped;
  an off-spec active task remains selected with a warning.
- `taskrail task new` — derive a slugged id and filename from `--title`
  (`T-<n>-<slug>`). Accented Latin text is folded to readable ASCII, title-derived
  slugs are capped, explicit `--slug` values are normalized but uncapped, and an
  empty normalized slug falls back to a warned bare id.
- `taskrail import --apply` and `taskrail verify --create-followup` — derive capped,
  title-based task ids and filenames; empty slugs keep the warned bare-id fallback.

### Fixed

- Spec anchor parsing now ignores ATX-looking lines inside backtick and tilde
  fences across validation, inspection, coverage, gap analysis, and spec diff.
- Release builds now consistently report a `v`-prefixed version, and publishing
  portably refuses missing or whitespace-only versioned changelog sections.
- Every command now refuses a repository recording a `layout_version` newer than
  the binary supports before reading or writing state, not only during `init`.
- `taskrail task new` and `task rename` now honor explicitly empty `--slug`
  selectors; title-derived slug caps preserve whole tokens and fall back to a bare id.
- `taskrail task rename` now refuses an invalid post-rename preview before writing
  the task, inbound dependencies, or `STATE.md`.
- State-writing notes, reasons, task titles, follow-ups, and imported drafts now
  reject concrete gitignored `planning/artifacts/` paths before writing committed
  content that a later `validate` would reject.
- `taskrail import --apply` now reports every file written or possibly touched by
  a failed apply, including failed spec writes and created tasks; JSON marks the
  result `"partial": true` while the command exits non-zero.
- `taskrail verify` and `taskrail block` now reuse `## Implementation Notes` in
  CRLF-authored tasks; the task-body guard ignores scaffold headings inside fenced
  examples and checks every scaffold section for duplicates.
- `taskrail task new`, `task repoint`, and `import --apply` now write a canonical
  `spec_ref` path and reject no-op repoints between equivalent spellings.
- Layout-marker and skill-backup errors now name repository-relative paths once,
  avoiding machine-specific absolute paths and duplicated error text.
- Installed-skill skew warnings now ignore adopter-owned skills outside Taskrail's
  embedded package, including nested skills under supported install targets.

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
