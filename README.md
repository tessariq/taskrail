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

**Turn goals into tracked work, kept aligned to one authoritative state file.**

Taskrail is a deterministic execution harness for humans and AI agents. It turns goals into structured tasks, keeps every transition aligned to a single authoritative state file, and advances work through validation, verification, and explicit follow-up.

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
- **State-first:** one authoritative `planning/STATE.md` is the continuity and control surface for all work.
- **Repo-native:** work is tracked as Markdown task files with an explicit, machine-checkable schema — specs under `specs/`, tracked work under `planning/`. No database, no hidden automation.
- **Verification is first-class:** completing implementation and verifying it are distinct steps; verification records pass/fail outcomes, writes inspectable artifacts, and opens follow-up tasks as needed.
- **Retrofit-friendly:** `taskrail init` (or `retrofit`) drops the contract into an existing repository with no rewrite.
- **Agent-ready:** every command has a `--json` path where it matters, so coding agents drive the same workflow humans do.

## What It Is Not

- Not a built-in LLM provider integration — Taskrail is provider-agnostic and manual-first. (`import` structures notes; it never calls a model.)
- Not a sandbox, container, or worktree orchestrator.
- Not a background daemon, distributed worker pool, or multi-lane scheduler.
- Not a *semantic* spec-to-task generator or drift detector — `import` produces structural drafts only; LLM-assisted generation is deferred.

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
go build -ldflags "-X main.version=v0.3.0" -o taskrail ./cmd/taskrail
# or, via Taskfile:
VERSION=v0.3.0 task release
./taskrail version   # -> v0.3.0
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

Every command takes `--json` where it matters, so agents drive the same loop.
Idle `next` selection is anchored to the active spec: it considers only `todo`
tasks whose `spec_ref` points at the active spec, so higher-priority older-spec
work is skipped rather than selected. When only older-spec work is runnable,
`next` reports no eligible task and lists the skipped tasks under `warnings`
(`skipped_non_active_spec`). An already-active task that points outside the
active spec is still returned so you can continue or resolve it, with a
`selected_non_active_spec` warning. Recover older work explicitly with
`start <id>`.

**Beyond the core loop**

- **Adopt an existing repo** — `init` and `retrofit` scaffold `specs/` + `planning/` non-destructively; `import` turns rough notes into spec/task drafts without an LLM; `repair` reconciles mechanical `STATE.md` drift.
- **See where work stands** — `status`, `stats`, and `coverage` report a live snapshot, aggregate metrics, and advisory spec-linkage, all read-only. `status` also breaks down open work (`todo`/`in_progress`/`blocked`) by how much targets the active spec versus points away from it, listing the away tasks and their `spec_ref`; the away set matches the active-spec filter `next` uses for idle selection.
- **Author and steer specs** — the `spec` family (`list`, `show`, `add`, `activate`) inspects and evolves versioned specs.
- **Handle the messy parts** — `block`/`unblock` park and resume work, `task new` scaffolds a task with the next free id, and `task rename` atomically re-slugs a task's id, filename, and inbound dependency references.

Run `taskrail --help`, or `taskrail <command> --help`, for the full command list and every flag.

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
`<path>#<anchor>` values for `task new --spec-ref`, and the active spec's bare
anchors for `task new --area` (the anchors it offers are exactly the ones
`validate` accepts, so a completed reference authors a task that passes
`validate`).

## Quickstart

Initialize Taskrail inside an existing repository, then confirm it is sane:

```sh
taskrail init
taskrail validate
```

Tasks live under `planning/tasks/` as Markdown with YAML frontmatter:

```md
---
id: T-001
title: Bootstrap repository structure
status: pending
priority: high
spec_ref: specs/v0.1.0.md#summary
dependencies: []
---

# T-001 Bootstrap repository structure

## Description

Create the initial Taskrail structure, specs, and planning area.

## Acceptance

- `planning/STATE.md` exists.
- `taskrail validate` passes.
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
taskrail task new --title "Add drift breakdown" --area status-active-spec-drift-breakdown
taskrail spec show v0.4.0 --anchors   # list the active spec's valid anchors
```

`--area` and `--spec-ref` are mutually exclusive; an unknown anchor fails before
anything is written and points you at `spec show <active-version> --anchors`.

### The slug-in-id invariant

A task's `id` and its filename are two encodings of one identifier: `validate`
enforces `filename == "<id>.md"`, so a slugged filename requires a slugged id.
`task new` produces that pairing directly — `--title "X"` derives a slug and
writes `T-<n>-x-slug` with a matching `T-<n>-x-slug.md`, `--slug` overrides the
slug source, and passing neither keeps the bare `T-<n>` / `T-<n>.md` form. Every
case passes `validate` with no follow-up edit.

Because the id and filename move together, you cannot rename a file for
readability on its own. A bare `git mv T-<n>.md T-<n>-add-slug.md` changes only
the filename, leaving the frontmatter `id:` as `T-<n>`, so the next `validate`
fails with `task <id> filename must be <id>.md`. The fix is `task rename`, which
re-slugs atomically: it rewrites the `id:` field, renames the file, rewrites
every inbound `dependencies:` reference to the task, re-projects `STATE.md`, and
re-runs `validate`.

```sh
taskrail task rename T-<n> --slug add-slug     # or --title "Add slug"; --dry-run previews
```

Bootstrap drafts from rough notes without any LLM — preview first, then apply:

```sh
taskrail import notes.md --to tasks                # preview the structural task drafts
taskrail import notes.md --to tasks --emit-prompt  # print an agent prompt for a richer draft
taskrail import --apply draft.json                 # validate an agent draft and write real files
```

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
  STATE.md                         # single authoritative state surface
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
state nor `validate` depends on it surviving a Git round-trip.

## State Contract

`planning/STATE.md` is the authoritative execution state. It carries the active spec, current task, status summary, blockers, the next action, and the last verification result, plus pointers to relevant artifacts. Do not hand-edit machine-managed state fields — let the `taskrail` transitions update them.

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
├── planning/          # authoritative tracked work and STATE.md
├── scripts/
└── specs/             # versioned, normative product specs
```

The packaged skill set lives in `internal/taskrail/skills/` (embedded; installed
by `taskrail init --with-skills`). This repository adopts it: committed copies in
`.agents/skills/` and `.claude/skills/` are kept byte-identical to the package by
`task check:skills`.

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
fails loudly if the on-PATH binary is stale versus the working tree.

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

- `pre-commit`: `gofmt`, `go vet ./...`, `taskrail validate`, skill package-parity check.
- `commit-msg`: Conventional Commit subject; rejects automated-attribution trailers.
- `pre-push`: `go test ./...`.

Hooks are a convenience; CI (`.github/workflows/ci.yml`) remains the authoritative
gate. Do not bypass them with `--no-verify`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the PR checklist, the AI-assisted
contribution policy, and tracked-work rules.

## Status

Taskrail is an in-progress open-source project. The current release is `v0.3.0`.

- `v0.1.0` established the repository contract: deterministic task progression, the authoritative `STATE.md`, and verification as a first-class concept.
- `v0.2.0` makes adoption in existing repositories easy — guided `retrofit`, LLM-free `import` of rough notes into spec/task drafts, opt-in shippable agent skills, a version-aware non-destructive `init`, and conservative `STATE.md` repair — while keeping the core CLI provider- and tooling-independent.
- `v0.3.0` adds read-only insight into tracked work — `status`, `stats`, and `coverage` — plus the `spec` command family for inspecting and authoring specs, `unblock` to release blocked tasks, and Windows install via WinGet.
- Later work is tracked under [`specs/README.md`](specs/README.md).

This repository also dogfoods the Taskrail workflow style — using `planning/`, `docs/workflow/`, and the packaged skill set it adopts like any adopter — until the product itself fully replaces that scaffolding.

## License

Apache-2.0. See [LICENSE](LICENSE).

## Read Next

- [`specs/v0.3.0.md`](specs/v0.3.0.md) — current release scope
- [`specs/README.md`](specs/README.md) — spec reading order and versioning
- [`planning/STATE.md`](planning/STATE.md) — live execution state
- [`AGENTS.md`](AGENTS.md) — guidance for coding agents
- [`CHANGELOG.md`](CHANGELOG.md)

The versioned specs in `specs/` remain the normative source of truth for release scope and behavior.
