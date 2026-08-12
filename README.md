<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo/lockup-horizontal-mono-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assets/logo/lockup-horizontal-mono-light.svg">
  <img alt="Taskrail" src="assets/logo/lockup-horizontal-mono-dark.svg" height="56">
</picture>

[![CI](https://github.com/tessariq/taskrail/actions/workflows/ci.yml/badge.svg)](https://github.com/tessariq/taskrail/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/tessariq/taskrail)](https://github.com/tessariq/taskrail/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/tessariq/taskrail)](https://github.com/tessariq/taskrail/blob/main/go.mod)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](https://github.com/tessariq/taskrail/blob/main/LICENSE)

# Taskrail

**Turn goals into tracked work, kept aligned through inspectable repository state.**

Taskrail is a deterministic execution harness for humans and AI agents. It turns goals into structured tasks, keeps lifecycle transitions aligned across task files and a generated current-state projection, and advances work through validation, verification, and explicit follow-up.

It is built on durable primitives — Git for history and review, plain Markdown with YAML frontmatter for specs, tasks, and state. No database. No hidden automation. No opaque dashboards. Your repo stays inspectable, and the same `taskrail` commands work whether a person or an agent is at the keyboard.

```sh
taskrail init          # adopt Taskrail in an existing repo, non-destructively
taskrail validate      # confirm the layout and state are consistent
taskrail status        # see the active spec, task counts, and what's next
```

From there, the daily loop is `next → start → complete → verify` (see [Commands](#commands)).

## Contents

- [Why Taskrail](#why-taskrail)
- [What It Is Not](#what-it-is-not)
- [Install](#install)
- [Commands](#commands)
- [Quickstart](#quickstart)
- [What a Verification Leaves Behind](#what-a-verification-leaves-behind)
- [State Contract](#state-contract)
- [Repository Layout](#repository-layout)
- [Development](#development)
- [Status](#status)
- [License](#license)
- [Read Next](#read-next)

## Why Taskrail

- **Deterministic:** the workflow is `validate → next → start → complete → verify`, and next-task selection follows status, dependencies, priority, and stable tie-breaking — same repo, same answer, every time.
- **State-first:** task files are the durable work ledger and `planning/STATE.md` is the generated continuity and control projection for the current run.
- **Repo-native:** work is tracked as Markdown task files with an explicit, machine-checkable schema — specs under `specs/`, tracked work under `planning/`. No database, no hidden automation.
- **Verification is first-class:** completing implementation and verifying it are distinct steps; verification records pass/fail outcomes, writes inspectable artifacts, and opens follow-up tasks as needed.
- **Retrofit-friendly:** `taskrail init` (or `retrofit`) drops the contract into an existing repository with no rewrite.
- **Agent-ready:** reporting and most agent handoffs have structured output today; the active v0.5 spec completes lifecycle JSON parity and standardizes one machine-result envelope.

## What It Is Not

- Not a built-in LLM provider integration — Taskrail is provider-agnostic and manual-first. (`import` structures notes; it never calls a model.)
- Not a sandbox, container, or worktree orchestrator.
- Not a background daemon, distributed worker pool, or multi-lane scheduler.
- Not a built-in *semantic* spec-to-task generator or reviewer — the binary provides mechanical reports and reviewed write boundaries, while optional skills let an external agent supply judgement.

## Install

Homebrew (macOS and Linux):

```sh
brew install tessariq/tap/taskrail
taskrail --version
```

This pulls the release binary from the [tessariq/homebrew-tap](https://github.com/tessariq/homebrew-tap) tap.

Windows (WinGet):

```sh
winget install Tessariq.Taskrail
taskrail --version
```

Build from source (needs Go `1.26`):

```sh
git clone https://github.com/tessariq/taskrail.git
cd taskrail
go install ./cmd/taskrail
taskrail version
```

Plain `go build`/`go install` produce a development build that reports version
`0.0.0-dev`. To produce a release build that reports a real version, inject it at
build time:

```sh
VERSION=vX.Y.Z
go build -ldflags "-X main.version=${VERSION}" -o taskrail ./cmd/taskrail
# or, via Taskfile:
VERSION="${VERSION}" task release
./taskrail version   # -> vX.Y.Z
```

Tagged `v*` releases are built and published automatically with
[GoReleaser](https://goreleaser.com) — Linux/macOS/Windows binaries for
`amd64`/`arm64`, with archives, checksums, and notes from `CHANGELOG.md`. See
[`docs/workflow/releasing.md`](docs/workflow/releasing.md) for the release checklist.

## Commands

The core loop is five commands — the ones you run every day:

```sh
taskrail validate                                    # check the repo is consistent
taskrail next --json                                 # pick the next eligible active-spec task
taskrail start T-001                                 # mark it active
taskrail complete T-001 --note "implemented"         # mark implementation done
taskrail verify T-001 --result pass --summary "acceptance met"
```

Every `--json` command — including the `start`, `complete`, and `block`
lifecycle writers — emits one versioned envelope: `schema_version`, the canonical
`command`, `warnings`, and exactly one of `result` or `error`. A failure carries a
registered `error.code` plus `details` recording whether the operation committed
and which paths it touched. This is the one-time v0.5 break from pre-v0.5 bare
result objects: consumers read command payloads under `result`, and there is no
legacy-output switch. `schema_version` versions the whole document, including
result, warning, error, enum, nullability, and semantic contracts, not only the
outer member names. Consumers must reject unsupported versions rather than decode
an inherited shape optimistically. Text output stays human-oriented and unchanged.
Idle `next` selection is anchored to the active spec: it considers only `todo`
tasks whose `spec_ref` points at the active spec, so higher-priority older-spec
work is skipped rather than selected. When only older-spec work is runnable,
`next` reports no eligible task and lists the skipped tasks under `warnings`
(`skipped_non_active_spec`). An already-active task that points outside the
active spec is still returned so you can continue or resolve it, with a
`selected_non_active_spec` warning. Recover older work explicitly with
`start <id>`, or run `next --include-off-spec` for a one-shot pick that ranks
`todo` tasks across all specs and flags an off-spec selection (`off_spec` /
`selected_off_spec`). To move an off-spec task *onto* the active spec instead of
running it where it is, use
[`task repoint`](#re-pointing-a-task-onto-another-spec-area).

**Beyond the core loop**

- **Adopt an existing repo** — `init` and `retrofit` scaffold `specs/` + `planning/` non-destructively; `import` turns rough notes into spec/task drafts without an LLM; `repair` reconciles mechanical `STATE.md` drift.
- **See where work stands** — `status`, `stats`, and `coverage` report a live snapshot, aggregate metrics, and advisory spec-linkage, all read-only. `status` also breaks down open work (`todo`/`in_progress`/`blocked`) by how much targets the active spec versus points away from it, listing the away tasks and their `spec_ref`; the away set matches the active-spec filter `next` uses for idle selection.
- **Author and steer specs** — the `spec` family (`list`, `show`, `add`, `activate`, `diff`) inspects and evolves versioned specs; `spec diff` previews the mechanical area-set delta before activation.
- **Draft missing work** — the optional `taskrail-decompose` and `taskrail-gap` skills turn uncovered areas and structural gap signals into reviewable proposals; only an explicit `task new` or `import --apply` writes tracked tasks.
- **Handle the messy parts** — `block`/`unblock` park and resume work, `task new` scaffolds a task with the next free id, `task rename` atomically re-slugs a task's id, filename, and inbound dependency references, and `task repoint` moves an open task's `spec_ref` onto another area.

Run `taskrail --help`, or `taskrail <command> --help`, for the full command list and every flag.

### Command effects

Taskrail commands intentionally use different write conventions based on risk:

| Class | Current examples | Effect |
|---|---|---|
| Read-only | `validate`, `status`, `stats`, `coverage`, `spec list/show/diff` | Inspect only; never rewrite tracked planning state. |
| Mode-dependent initialization | `init` | Fresh, unmarked-standard, and current-layout adoption/repair paths may write immediately; detected migration or retrofit paths preview unless `--apply` is supplied. In v0.4, `--with-skills` may also install skills after any successful init result. |
| Preview by default | `retrofit`, `repair` | Report a candidate; `--apply` is the write opt-in. |
| Apply with preview option | `task rename`, `task repoint` | Write by default; `--dry-run` validates the candidate first. |
| Lifecycle/state writers | `next`, `start`, `complete`, `block`, `unblock`, `verify`, `spec activate`, `task new` | Rewrite `STATE.md` and sometimes task files; inspect `git status` afterward. |
| Reviewed import writer | `import --apply <draft>` | Validates an external draft and writes its bounded task/spec/state set. |

`next` is not a read-only selection probe: it persists `next_action` and
`updated_at`. Use `status` when you need the same next-task computation without a
tracked write.

### Coverage vs gap analysis

`coverage` and `coverage --gaps` sit one word apart and answer different questions —
keep them distinct:

- `coverage` answers **"is this spec area linked to any task?"** — decomposition
  coverage, orphan tasks, and two-directional drift.
- `coverage --gaps` answers **"does a *covered* area lack a verification/companion
  task, have a dependency-graph anomaly, or look under-decomposed?"** — it emits
  structural candidates (`missing-verification`, `dependency-anomaly`,
  `under-decomposed-area`) over areas that already have tasks.

Both are **read-only** — they never write `STATE.md` or task files and never make
`validate` fail — and **advisory** by default. `--gaps` opts into gating only through
`--fail-on <category>`, which exits non-zero when a signal of that category is
present (mirroring `coverage --min`); the report itself is unchanged.

The hard limit: `--gaps` is **mechanical only**. It reports count, graph, and state
signals — never a semantic "this needs a test" judgement. Its signals are
**candidates, not violations**: false positives are expected, and each one is
something a human or agent inspects and promotes into a real task, not a rule the
repo broke. For the semantic half — "is this area *actually* missing work?" — use
the `taskrail-gap` skill, which layers agent judgement on top of these structural
candidates.

### Review stages

Taskrail keeps mechanically testable state separate from agent or human semantic
review. Current `coverage`/`coverage --gaps` report structure; `validate` checks
repository invariants; `verify` records evidence against one task. The active
v0.5 roadmap adds distinct advisory stages rather than one overloaded "review":

- post-spec consistency, gap, addition, and adversarial lenses before decomposition;
- one existing-task review for alignment, dependencies, acceptance, and evidence;
- adversarial review of an unpublished decomposed task set;
- separate implementation review before completion and passing verification; and
- post-implementation workflow-adversarial probes with bounded review memory.

Semantic findings never become `validate` violations automatically. Humans adopt
accepted changes through the bounded task/spec/import commands. Publisher-backed
review prompts may be replaced whole-file at repository scope; their durable leaf
artifacts record the exact prompt template resolution without claiming that
Taskrail observed or certified the external review process. Implementation review
remains instructed by the separate `task-implementation` flow and is outside
review-artifact publication.

### Shell completion

Taskrail ships shell completion via Cobra. Load it for your shell (or add the
line to your shell profile):

```sh
source <(taskrail completion bash)   # bash
taskrail completion zsh > "${fpath[1]}/_taskrail"   # zsh
taskrail completion fish | source   # fish
```

Run `taskrail completion --help` for per-shell install steps. Completion is
read-only: it never writes `STATE.md` or task files. Beyond every command and
flag, it completes spec versions for `spec show`/`spec activate`, real
`<path>#<anchor>` values for `task new --spec-ref` and `task repoint --spec-ref`,
and the active spec's bare anchors for their `--area` flags (the anchors it offers
are exactly the ones `validate` accepts, so a completed reference authors or
re-points a task that passes `validate`).

## Quickstart

Initialize Taskrail inside an existing repository, then confirm it is sane:

```sh
taskrail init --apply
taskrail validate
```

Tasks live under `planning/tasks/` as Markdown with YAML frontmatter:

```md
---
id: T-001
title: Bootstrap repository structure
status: todo
priority: high
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-08-05T00:00:00Z"
---

# T-001 Bootstrap repository structure

## Description

Create the initial Taskrail structure, specs, and planning area.

## Acceptance

- `planning/STATE.md` exists.
- `taskrail validate` passes.

## Verification Notes

- Run `taskrail validate` and record the successful observation.

## Implementation Notes
```

Let Taskrail pick the next eligible task, start it, and advance it:

```sh
taskrail next --json
taskrail start T-001
taskrail complete T-001 --note "implementation landed"
taskrail verify T-001 --result pass --summary "validate passes; acceptance met"
```

When verification reveals more work, spawn a follow-up task in the same step:

```sh
taskrail verify T-001 \
  --result fail \
  --summary "missing dependency check" \
  --create-followup \
  --followup-title "Add dependency validation" \
  --followup-priority high
```

Author a task against the active spec without copying the spec path by hand —
`--area <anchor>` is shorthand for `--spec-ref <active-spec-path>#<anchor>`:

```sh
taskrail task new --title "Add machine envelope" --area uniform-agent-machine-results
taskrail spec show v0.5.0 --anchors   # list the active spec's valid anchors
```

`--area` and `--spec-ref` are mutually exclusive; an unknown anchor fails before
anything is written and points you at `spec show <active-version> --anchors`.

Before advancing the active spec, inspect what changed between two versions with a
read-only, mechanical anchor-set delta:

```sh
taskrail spec diff v0.3.0 v0.4.0   # added / removed / candidate-rename areas
```

Added areas are the ones a migration must decompose into tasks; removed areas are
the ones whose existing tasks become orphaned drift; rename candidates are
best-effort only (an added and a removed anchor sharing a normalized stem) and
labeled for you to verify, never asserted as fact. Like `coverage` and `validate`
it is side-effect-free: it never writes `STATE.md` or task files and never gates
validation. `--json` mirrors the output with structured `added`/`removed`/`renamed`
lists.

### The slug-in-id invariant

A task's `id` and its filename are two encodings of one identifier: `validate`
enforces `filename == "<id>.md"`, so a slugged filename requires a slugged id.
`task new` produces that pairing directly — `--title "X"` derives a slug and
writes `T-<n>-x-slug` with a matching `T-<n>-x-slug.md`, `--slug` overrides the
slug source, and passing neither keeps the bare `T-<n>` / `T-<n>.md` form. Every
case passes `validate` with no follow-up edit. Accented letters transliterate to
ASCII first, so `--title "Über Fußball"` yields `T-<n>-ueber-fussball` and
`--title "Łódź Điện"` yields `T-<n>-lodz-dien` rather than a mangled slug — however
your keyboard encoded the accent. Title-derived slugs keep only complete tokens up
to the roughly 50-character cap; if the first token alone exceeds the cap, the
bounded bare-id fallback is used instead of cutting that token. If the value you
pass normalizes to no slug at all (`--slug ""`, `--slug "!!!"`,
a fully non-Latin title), the bare `T-<n>` id is written and a warning naming the
source goes to stderr — `--json` on stdout stays clean.

Because the id and filename move together, you cannot rename a file for
readability on its own. A bare `git mv T-<n>.md T-<n>-add-slug.md` changes only
the filename, leaving the frontmatter `id:` as `T-<n>`, so the next `validate`
fails with `task <id> filename must be <id>.md`. The fix is `task rename`, which
re-slugs atomically: it rewrites the `id:` field, renames the file, repoints the
body's `# <id> <title>` heading, rewrites every inbound `dependencies:` reference
to the task, re-projects `STATE.md`, and re-runs `validate`.

```sh
taskrail task rename T-<n> --slug add-slug     # or --title "Add slug"; --dry-run previews
```

Rename is symmetric with creation: an explicitly empty selector, or one that
normalizes to no slug, strips
the slug instead of failing, renaming `T-<n>-<slug>.md` back to `T-<n>.md` (with
the same stderr warning), so a bad slug can be undone. The length cap is
symmetric too — a `--title`-derived slug is capped the same way `task new`
caps it, while an explicit `--slug` is normalized but not length-capped.

`task rename` re-encodes the identifier only: it changes the id/slug and filename
but never rewrites the `title:` frontmatter field. Re-slugging a task and
retitling its human-readable title are distinct operations, and there is no
`task retitle` command in this version — so `task rename --title "New Title"`
derives a new slug and leaves the title unchanged, by design. To change the
visible title, edit the task's `title:` field directly.

`--dry-run` writes nothing, and its reported validation previews the state the
rename *would* leave behind, not the one it would replace — so re-slugging to heal
a `filename must be <id>.md` drift answers "would this fix it?". The preview covers
the whole change set (id, filename, inbound dependency refs, and the `current_task`
pointer when it names the task); violations the rename does not touch still show up.

### Re-pointing a task onto another spec area

After `spec activate`, open tasks still pointing at the previous spec are off-spec:
`next` skips them, `status` lists them under the active-spec drift breakdown, and
`next --include-off-spec` recovers one to run where it is. To move an open task
*onto* the active spec instead, `task repoint` rewrites its `spec_ref` — the one
edit that would otherwise mean hand-editing frontmatter.

```sh
taskrail task repoint T-<n> --area status-active-spec-drift-breakdown  # active-spec anchor
taskrail task repoint T-<n> --spec-ref specs/v0.2.0.md#some-area       # explicit, cross-spec
taskrail task repoint T-<n> --area some-area --dry-run                 # preview, writes nothing
```

`--area` resolves the anchor against `STATE.md`'s active spec exactly as `task new
--area` does, so an unknown anchor fails before any write and points at `spec show
<active-version> --anchors`. `--area` and `--spec-ref` are mutually exclusive.

`--dry-run` writes nothing, and its reported validation previews the state the
repoint *would* leave behind, not the one it would replace — so previewing a fix
for a broken `spec_ref` answers "would this make the repo valid?". Violations the
repoint does not touch still show up.

Repoint re-encodes one reference field: it never touches the id, slug, filename,
title, status, or dependencies, and never rewrites another task file. It is not a
status mutator and not a bulk migrator. Completed and cancelled tasks are delivered
history and are rejected. Because it re-projects `planning/STATE.md`, run `git
status` afterwards and stage the regenerated file with the change.

Bootstrap drafts from rough notes without any LLM — preview first, then apply:

```sh
taskrail import notes.md --to tasks                # preview the structural task drafts
taskrail import notes.md --to tasks --emit-prompt  # print an agent prompt for a richer draft
taskrail import --apply draft.json                 # validate an agent draft and write real files
```

An apply that fails during writing exits non-zero and still reports what it wrote
or may have touched — the spec and task paths in text mode, and with `--json` a
`partial_write` error whose `details.paths` name them. Review those paths before
retrying: a failed spec write may leave an empty or truncated file, and
re-applying the same draft creates any already-written tasks a second time under
new ids.

Typical flow:

1. Write a goal as a Markdown task inside `planning/tasks/`.
2. `validate` the repository.
3. `next` to select deterministically, then `start`.
4. `complete` the implementation.
5. `verify` to record the outcome and leave artifacts — opening follow-up tasks as needed.

## What a Verification Leaves Behind

Every verification writes repo-local evidence under `planning/artifacts/verify/<task-id>/<timestamp>/`:

```text
planning/
  STATE.md                         # generated current execution projection
  NOTES.md                         # optional human-owned repository context
  tasks/
    T-001.md                       # task with frontmatter schema
  artifacts/
    verify/
      T-001/
        20260619T113646Z/
          plan.md                  # verification plan
          report.json              # machine-readable outcome
          report.md                # human-readable outcome
```

These are plain files — no proprietary formats, no database required. The
`planning/artifacts/` tree is gitignored, reproducible local output: `verify`
creates it on demand, `taskrail init` never pre-creates it, and neither committed
state nor `validate` depends on it surviving a Git round-trip. No `.gitkeep`
placeholder is required or tracked.

## State Contract

`planning/STATE.md` is the authoritative current execution projection. It carries the active spec, current task, status summary, blockers, the next action, and the last verification result, plus pointers to relevant artifacts. It is not a per-task or per-session log: keep durable task context in task `## Implementation Notes`, blocker reasons, portable verification summaries/reports, or follow-up tasks. Repository-wide human context lives in `planning/NOTES.md`, a human-owned sidecar `init` and `retrofit --apply` create as a short commented template when that path is absent and never rewrite afterwards; agents may read it but edit it only when explicitly asked. Do not hand-edit machine-managed state fields or append continuation prose; let the `taskrail` transitions update the file.

## Repository Layout

```text
.
├── AGENTS.md          # guidance for coding agents
├── CHANGELOG.md
├── README.md
├── cmd/taskrail/      # CLI entry point
├── internal/          # core packages
├── lefthook.yml       # opt-in local git hooks (mirror CI)
├── mise.toml          # optional pinned developer toolchain (mise)
├── planning/          # task ledger, generated STATE.md, optional human NOTES.md
├── scripts/
└── specs/             # versioned, normative product specs
```

The packaged skill set lives in `internal/taskrail/skills/` (embedded; installed
by `taskrail init --with-skills`). This repository adopts it: committed copies in
`.agents/skills/` and `.claude/skills/` are kept byte-identical to the package by
`task check:skills`. Installed skills additionally record the Taskrail version
that wrote them in `metadata.taskrail_version`, so an install left behind
by an older binary stays detectable: every command warns on stderr when the
recorded version is not the running one, naming the affected skills and both
versions. The warning is advisory — it never fails `validate` or blocks a
transition — and `taskrail init --with-skills --force` resolves it. The committed
copies in this repository carry no marker, since parity keeps them byte-identical
to the unstamped package; byte-identical marker-free copies are silent rather than
reported as unknown-version. Do not run `--force` here, since stamping the
committed copies would break `task check:skills`.

Readers retain the legacy top-level marker written by released v0.4 binaries, and
an explicit successful refresh normalizes it to nested metadata. Maintainer skill
eval cases remain outside the embedded package and are not installed into adopter
repositories.

## Development

[mise](https://mise.jdx.dev) can pin and provision the developer toolchain (Go,
`task`, `lefthook`) from the committed `mise.toml`. It is optional convenience —
direct `go` commands and the `Taskfile.yml` targets work without it:

```sh
mise install     # provision the pinned toolchain on a fresh clone
mise run setup   # provision, build taskrail onto PATH, wire the opt-in git hooks
```

`mise run setup` (and `task taskrail:install`) build the working-tree
`./cmd/taskrail` into `./bin` and mise puts `./bin` on PATH, so a bare `taskrail`
resolves to the current build with no `TASKRAIL` override. `task taskrail:check`
fails loudly if the on-PATH binary is stale versus the working tree. In this
source checkout, run that check immediately before `${TASKRAIL:-taskrail}` state
writers; the packaged skills carry the same source-only guard. Adopter
repositories without Taskrail's build tooling are unaffected.

The `mise.toml` pins are the single source of truth: the `go` pin matches `go.mod`
and the `lefthook` pin matches the hooks guidance below. CI provisions the same
toolchain via [`jdx/mise-action`](https://github.com/jdx/mise-action), so local and
CI builds share one set of pinned versions. The build/test job runs as an OS matrix
over Linux, Windows, and macOS, catching cross-platform regressions (path
separators, line endings, file modes) before merge.

Optional git hooks mirror the CI checks locally via
[lefthook](https://github.com/evilmartians/lefthook). `mise run setup` wires them;
to install by hand:

```sh
go install github.com/evilmartians/lefthook@v1.13.6   # or: brew install lefthook
task hooks:install
```

- `pre-commit`: `gofmt`, `go vet ./...`, `taskrail validate`, skill package-parity check,
  `task taskrail:check` (the on-PATH `taskrail` must be the current working-tree build).
- `commit-msg`: Conventional Commit subject; rejects automated-attribution trailers.
- `pre-push`: `go test ./...`.

Hooks are a convenience; CI (`.github/workflows/ci.yml`) remains the authoritative
gate. Do not bypass them with `--no-verify`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the PR checklist, the AI-assisted
contribution policy, and tracked-work rules.

## Status

Taskrail is an in-progress open-source project. The current release is `v0.4.0`;
the active development specification is `v0.5.0`.

- `v0.1.0` established the repository contract: deterministic task progression, the authoritative `STATE.md`, and verification as a first-class concept.
- `v0.2.0` makes adoption in existing repositories easy — guided `retrofit`, LLM-free `import` of rough notes into spec/task drafts, opt-in shippable agent skills, a version-aware non-destructive `init`, and conservative `STATE.md` repair — while keeping the core CLI provider- and tooling-independent.
- `v0.3.0` adds read-only insight into tracked work — `status`, `stats`, and `coverage` — plus the `spec` command family for inspecting and authoring specs, `unblock` to release blocked tasks, and Windows install via WinGet.
- `v0.4.0` adds active-spec task selection and authoring, slugged task creation and atomic rename/repoint operations, mechanical spec/gap review, and version-skew detection across the binary, repository layout, and installed skills.
- `v0.5.0` is active development: uniform agent results, lifecycle-complete skills, human-owned repository notes, configurable prompt/task/spec review, prompt-bound safe review publication, a bounded external-process loop, and maintainer-run skill evaluations.
- `v0.6.0` plans durable task identity, cancellation preview, dependency editing, all-or-none legacy imports, and immutable archival over one ledger.
- `v0.7.0` plans reviewed OpenSpec/Spec Kit handoff, immutable planning-source receipts, and profile/receipt inventory.
- Later work is tracked under [`specs/README.md`](specs/README.md).

This repository also dogfoods the Taskrail workflow style — using `planning/`, `docs/workflow/`, and the packaged skill set it adopts like any adopter — until the product itself fully replaces that scaffolding.

## License

Apache-2.0. See [LICENSE](LICENSE).

## Read Next

- [`specs/v0.4.0.md`](specs/v0.4.0.md) — current release scope
- [`specs/README.md`](specs/README.md) — spec reading order and versioning
- [`planning/STATE.md`](planning/STATE.md) — live execution state
- [`AGENTS.md`](AGENTS.md) — guidance for coding agents
- [`CHANGELOG.md`](CHANGELOG.md)

The versioned specs in `specs/` remain the normative source of truth for release scope and behavior.
