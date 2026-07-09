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
taskrail init                                        # adopt Taskrail in an existing repo
taskrail next --json                                 # pick the next eligible task, deterministically
taskrail start T-001
taskrail complete T-001 --note "implemented"
taskrail verify T-001 --result pass --summary "acceptance met"
```

## Contents

- [Why Taskrail](#why-taskrail)
- [What It Is](#what-it-is)
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

- **Deterministic:** next-task selection follows status, dependencies, priority, and stable tie-breaking — same repo, same answer, every time.
- **State-first:** one authoritative `planning/STATE.md` is the continuity and control surface for all work.
- **Verification as a first-class concept:** completing implementation and verifying it are distinct steps, and verification leaves durable artifacts.
- **Retrofit-friendly:** `taskrail init` (or `retrofit`) drops the contract into an existing repository with no rewrite.
- **Agent-ready:** every command has a `--json` path where it matters, so coding agents drive the same workflow humans do.

## What It Is

- A CLI for tracking repo-native work as Markdown task files with an explicit, machine-checkable schema.
- A deterministic workflow: `validate → next → start → complete → verify`.
- One authoritative `planning/STATE.md` — the continuity and control surface for all work.
- Repo-native specs under `specs/` and deterministic tracked work under `planning/`.
- Task validation (dependency checks, spec references), deterministic next-task selection, and explicit transitions.
- A verification model that records pass/fail outcomes, writes inspectable artifacts, and opens follow-up tasks as needed.

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
go build -ldflags "-X main.version=v0.2.0" -o taskrail ./cmd/taskrail
# or, via Taskfile:
VERSION=v0.2.0 task release
./taskrail version   # -> v0.2.0
```

Tagged releases are automated with [GoReleaser](https://goreleaser.com): pushing a
`v*` tag builds `linux`/`darwin` binaries for `amd64`/`arm64` and publishes archives
plus checksums to a GitHub Release, with notes taken from the matching `## v<version>`
section of `CHANGELOG.md`. The Homebrew tap is bumped by hand afterward — see
[`docs/workflow/releasing.md`](docs/workflow/releasing.md) for the full checklist.

## Commands

Run `taskrail <command> --help` for full flag details.

| Command | Purpose |
| --- | --- |
| `taskrail init` | Version-aware, non-destructive init/upgrade for empty, existing, or non-standard repos. `--with-skills` installs the agent skills; `--apply` writes migrations/retrofits; `--json`. |
| `taskrail retrofit [notes]` | Guided bootstrap for a non-standard repo: detect layout, import notes into a reviewable draft, scaffold `specs/` + `planning/`. Dry run by default; `--apply`, `--emit-prompt`, `--json`. |
| `taskrail validate` | Validate layout, task shape, dependency/spec references, and `STATE.md` consistency. Read-only. |
| `taskrail repair` | Reconcile mechanical `STATE.md` drift (stale `current_task` pointer or task counts). Dry run by default; `--apply` rewrites `STATE.md` only; `--json`. |
| `taskrail coverage` | Report advisory spec-coverage percentage, orphan tasks, and drift for the active spec. Read-only; never fails `validate`. `--json`. |
| `taskrail status` | Print the current tracked-work snapshot: active spec, task counts, next eligible task (computed but not persisted), blocked tasks with reasons, last verification, and a one-line coverage summary. Read-only. `--json`. |
| `taskrail stats` | Report aggregate statistics computed snapshot-only from current task files and `STATE.md`: counts and percentages by status, blocked ratio and recorded-blocker count, spec coverage with a per-area breakdown, and dependency shape (unmet dependencies, longest chain). Read-only. `--json`. |
| `taskrail next` | Deterministically select the next eligible task. `--json`. |
| `taskrail start <task-id>` | Mark a task active and update `STATE.md`. |
| `taskrail complete <task-id>` | Mark a task implementation-complete. `--note`. |
| `taskrail block <task-id>` | Mark a task blocked and record a `--reason`. |
| `taskrail verify <task-id>` | Record a pass/fail outcome and write artifacts under `planning/artifacts/verify/`. `--result`, `--summary`, `--create-followup`, `--json`. |
| `taskrail spec activate <version>` | Repoint `STATE.md`'s active spec to a versioned target (e.g. `v0.3.0`), re-render `STATE.md`, and re-validate. CLI-only writer of the active spec; rejects a missing or non-conforming version with no write. `--json`. |
| `taskrail task new` | Scaffold a task file with the next free id. Requires `--title` and `--spec-ref`; `--priority`, repeatable `--dep`, `--follow-up <parent-id>`, `--json`. |
| `taskrail import <source>` | Build spec/task drafts from a markdown source without an LLM. `--to tasks\|spec\|planning`, `--emit-prompt`, `--apply <draft.json>`, `--json`. |
| `taskrail version` | Print the CLI version (also `--version`). |

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
├── skills/            # workflow skills (dogfooded until the product replaces them)
└── specs/             # versioned, normative product specs
```

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

- `pre-commit`: `gofmt`, `go vet ./...`, `taskrail validate`, skill-mirror check.
- `commit-msg`: Conventional Commit subject; rejects automated-attribution trailers.
- `pre-push`: `go test ./...`.

Hooks are a convenience; CI (`.github/workflows/ci.yml`) remains the authoritative
gate. Do not bypass them with `--no-verify`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the PR checklist, the AI-assisted
contribution policy, and tracked-work rules.

## Status

Taskrail is an in-progress open-source project. The current release is `v0.2.0`.

- `v0.1.0` established the repository contract: deterministic task progression, the authoritative `STATE.md`, and verification as a first-class concept.
- `v0.2.0` makes adoption in existing repositories easy — guided `retrofit`, LLM-free `import` of rough notes into spec/task drafts, opt-in shippable agent skills, a version-aware non-destructive `init`, and conservative `STATE.md` repair — while keeping the core CLI provider- and tooling-independent.
- Later work is tracked under [`specs/v0.3.0.md`](specs/v0.3.0.md).

This repository also dogfoods the Taskrail workflow style — using `planning/`, `docs/workflow/`, and mirrored skills — until the product itself fully replaces that scaffolding.

## License

Apache-2.0. See [LICENSE](LICENSE).

## Read Next

- [`specs/v0.2.0.md`](specs/v0.2.0.md) — current release scope
- [`specs/README.md`](specs/README.md) — spec reading order and versioning
- [`planning/STATE.md`](planning/STATE.md) — live execution state
- [`AGENTS.md`](AGENTS.md) — guidance for coding agents
- [`CHANGELOG.md`](CHANGELOG.md)

The versioned specs in `specs/` remain the normative source of truth for release scope and behavior.
