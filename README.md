<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo/lockup-horizontal-mono-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assets/logo/lockup-horizontal-mono-light.svg">
  <img alt="Taskrail" src="assets/logo/lockup-horizontal-mono-dark.svg" height="56">
</picture>

[![CI](https://github.com/tessariq/taskrail/actions/workflows/ci.yml/badge.svg)](https://github.com/tessariq/taskrail/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/tessariq/taskrail)](https://github.com/tessariq/taskrail/blob/main/go.mod)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](https://github.com/tessariq/taskrail/blob/main/LICENSE)

# Goals become tracked work. State stays authoritative.

Taskrail is a deterministic execution harness for humans and AI agents. It turns goals into structured tasks, keeps every transition aligned to one authoritative state file, and advances work through validation, verification, and explicit follow-up.

The project is built around durable primitives: Git for history and review, and plain Markdown with YAML frontmatter for specs, tasks, and state. No database. No hidden automation. No opaque dashboards. Your repo stays inspectable, and the same `taskrail` commands work whether a person or an agent is at the keyboard.

## Why Taskrail

- **Deterministic:** next-task selection follows status, dependencies, priority, and stable tie-breaking — same repo, same answer, every time.
- **State-first:** one authoritative `planning/STATE.md` is the continuity and control surface for all work.
- **Verification as a first-class concept:** completing implementation and verifying it are distinct steps, and verification leaves durable artifacts.
- **Retrofit-friendly:** `taskrail init` drops the contract into an existing repository with no rewrite.
- **Agent-ready:** every command has a `--json` path where it matters, so coding agents drive the same workflow humans do.

## What It Is

- A CLI for tracking repo-native work as Markdown task files with explicit, machine-checkable schema.
- A deterministic workflow built around `validate -> next -> start -> complete -> verify`.
- An authoritative state model centered on a single `planning/STATE.md`.
- A verification model that records pass/fail outcomes and writes inspectable artifacts.

## What It Is Not

- Not a built-in LLM provider integration — `v0.1.0` is provider-agnostic and manual-first.
- Not a sandbox, container, or worktree orchestrator.
- Not a background daemon, distributed worker pool, or multi-lane scheduler.
- Not a spec-to-task generator or semantic drift detector (yet).

## What Taskrail Owns

- repo-native specs under `specs/`
- deterministic tracked work under `planning/`
- one authoritative `planning/STATE.md`
- task validation, dependency checks, and spec references
- deterministic next-task selection
- explicit task transitions
- verification artifacts and follow-up tasks

## Install

Homebrew (macOS and Linux):

```sh
brew install tessariq/tap/taskrail
taskrail --version
```

This pulls the release binary from the [tessariq/homebrew-tap](https://github.com/tessariq/homebrew-tap) tap.

Build from source:

```sh
git clone https://github.com/tessariq/taskrail.git
cd taskrail
go install ./cmd/taskrail
taskrail version
taskrail --help
```

If you prefer a local binary in the repository directory:

```sh
go build ./cmd/taskrail
./taskrail version
./taskrail --help
```

Plain `go build`/`go install` produce a development build that reports version
`0.0.0-dev`. To produce a release build that reports a real version, inject it at
build time:

```sh
go build -ldflags "-X main.version=v0.1.0" -o taskrail ./cmd/taskrail
# or, via Taskfile:
VERSION=v0.1.0 task release
./taskrail version   # -> v0.1.0
```

Building from source needs Go `1.26`.

### Releases

Tagged releases are automated with [GoReleaser](https://goreleaser.com). Pushing a
`v*` tag triggers `.github/workflows/release.yml`, which builds `linux`/`darwin`
binaries for `amd64`/`arm64`, injects the tag into the version, and publishes
archives plus checksums to a GitHub Release. Release notes are taken from the
matching `## v<version>` section of `CHANGELOG.md`; a pre-publish guard fails the
release if that section is missing, so update the changelog before tagging.

Run `workflow_dispatch` on the Release workflow to build a `--snapshot` (no
publish), or validate locally:

```sh
goreleaser check
goreleaser release --snapshot --clean
```

## Commands

| Command | Purpose |
| --- | --- |
| `taskrail init` | Initialize Taskrail structure (`specs/`, `planning/`, starter `STATE.md`) in the current repository. |
| `taskrail validate` | Validate folder layout, required files, task shape, dependency and spec references, and `STATE.md` consistency. |
| `taskrail next` | Deterministically select the next eligible task. Supports `--json`. |
| `taskrail start <task-id>` | Mark a task as active and update `planning/STATE.md`. |
| `taskrail complete <task-id>` | Mark a task completed from an implementation perspective. Supports `--note`. |
| `taskrail block <task-id>` | Mark a task blocked and record a `--reason`. |
| `taskrail verify <task-id>` | Record a verification outcome and write artifacts under `planning/artifacts/verify/`. Supports `--result`, `--summary`, `--create-followup`, and `--json`. |
| `taskrail task new` | Scaffold a new task file with the next free id and a template body. Requires `--title` and `--spec-ref`; supports `--priority`, repeatable `--dep`, and `--json`. Refuses to write an invalid task (unknown spec anchor, nonexistent dependency). |
| `taskrail version` | Print the CLI version (also `--version`). |

## Quickstart

Initialize Taskrail inside an existing repository:

```sh
taskrail init
```

Confirm the repository is in a sane state:

```sh
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

These artifacts are plain files. No proprietary formats. No database required.

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
├── planning/          # authoritative tracked work and STATE.md
├── scripts/
├── skills/            # workflow skills (dogfooded until the product replaces them)
└── specs/             # versioned, normative product specs
```

## Development

Optional git hooks mirror the CI checks locally. They use
[lefthook](https://github.com/evilmartians/lefthook) and are opt-in:

```sh
go install github.com/evilmartians/lefthook@v1.13.6   # or: brew install lefthook
task hooks:install
```

- `pre-commit`: `gofmt`, `go vet ./...`, `taskrail validate`, skill-mirror check.
- `commit-msg`: Conventional Commit subject; rejects automated-attribution trailers.
- `pre-push`: `go test ./...`.

Hooks are a convenience; CI (`.github/workflows/ci.yml`) remains the authoritative
gate. Do not bypass them with `--no-verify`.

## Status

Taskrail is an in-progress open-source project centered on its first shippable release, `v0.1.0`.

- `v0.1.0` proves the repository contract, deterministic task progression, the authoritative `STATE.md`, and verification as a first-class concept.
- `v0.1.0` is manual-first and LLM-provider-agnostic; `loop`, retrofit content generation, and built-in LLM calls are explicitly out of scope.
- Later versions are tracked under `specs/v0.2.0.md` and `specs/v0.3.0.md`.

This repository also dogfoods the Taskrail workflow style — using `planning/`, `docs/workflow/`, and mirrored skills — until the product itself fully replaces that scaffolding.

## Read Next

- [`specs/v0.1.0.md`](specs/v0.1.0.md)
- [`specs/README.md`](specs/README.md)
- [`planning/STATE.md`](planning/STATE.md)
- [`AGENTS.md`](AGENTS.md)
- [`CHANGELOG.md`](CHANGELOG.md)

The versioned specs in `specs/` remain the normative source of truth for release scope and behavior.
