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

**Keep long-running work on the rails — in your repo, in plain Markdown, for humans and AI agents alike.**

## The Problem

An agent finishes a session and the plan is gone. The next session re-derives it,
picks a task someone already did, or declares work "done" with nothing to show
for it. You get plausible-looking progress and no way to audit it.

Chat history is not a work ledger. Neither is an issue tracker your agent cannot
reach, or a `TODO.md` nobody validates.

Taskrail puts the ledger in the repository: tracked work as Markdown files with a
machine-checkable schema, a generated current-state projection, and a CLI that
owns every status transition. Same repo, same question, same answer — whether a
person or an agent is at the keyboard. Git supplies history and review. There is
no database, no daemon, and no hidden automation.

```sh
taskrail init          # adopt Taskrail in an existing repo, non-destructively
taskrail validate      # confirm the layout and state are consistent
taskrail status        # active spec, task counts, and what's next
```

## Use Cases

### 1. Drive an AI agent through tracked work without losing the plot

The agent asks the CLI what to do next instead of guessing, and cannot fake a
transition — every status change goes through a command that rewrites the
ledger.

```sh
taskrail next --json                                 # deterministic selection
taskrail start T-001                                 # mark it active
taskrail complete T-001 --note "implemented"         # implementation done
taskrail verify T-001 --result pass --summary "acceptance met"
```

Selection follows status, dependencies, priority, and stable tie-breaking, and is
anchored to the active spec — so a fresh session lands on the same task a stale
one would. Every `--json` command emits one versioned envelope
(`schema_version`, `command`, `warnings`, and exactly one of `result` or
`error`), so an agent parses one shape.

For unattended runs, `loop` executes explicitly allowed tasks inside operator-set
bounds — see [docs/loop.md](docs/loop.md).

### 2. Adopt it in a repository that already has work in flight

Nothing is rewritten and nothing is generated behind your back.

```sh
taskrail init --apply                    # scaffold specs/ + planning/
taskrail import notes.md --to tasks      # rough notes -> task drafts, no LLM
taskrail import --apply draft.json       # validate a draft and write real files
```

`retrofit` handles messier existing layouts, `repair` reconciles mechanical
`STATE.md` drift, and `--with-skills` installs the packaged agent skills. `import`
never calls a model: it structures what you wrote, and `--emit-prompt` hands an
agent a prompt for a richer draft you then review.

### 3. Make "done" mean something

Completing implementation and verifying it are separate steps. Verification
records a pass/fail outcome, writes inspectable evidence, and can open the
follow-up task in the same breath:

```sh
taskrail verify T-001 \
  --result fail \
  --summary "missing dependency check" \
  --create-followup \
  --followup-title "Add dependency validation" \
  --followup-priority high
```

Each run leaves plain files under
`planning/artifacts/verify/<task-id>/<timestamp>-<verification-id>/` —
`plan.md`, `report.json`, `report.md`. That tree is gitignored, reproducible
local output: committed state never depends on it surviving a Git round-trip.

### 4. See where the work actually stands

All read-only, all safe to run mid-session:

```sh
taskrail status      # live snapshot, incl. work pointing away from the active spec
taskrail stats       # aggregate metrics and dependency graphs
taskrail coverage    # which spec areas have no task linked
```

`coverage --gaps` extends this with mechanical structural-gap *candidates* —
never violations, and never a reason `validate` fails.

## What It Is Not

- Not a built-in LLM provider integration — Taskrail is provider-agnostic and
  manual-first.
- Not a sandbox, container, or worktree orchestrator.
- Not a background daemon, distributed worker pool, or multi-lane scheduler.
- Not a built-in *semantic* spec-to-task generator or reviewer — the binary
  provides mechanical reports and reviewed write boundaries, while optional
  skills let an external agent supply judgement.

## Install

```sh
brew install tessariq/tap/taskrail     # macOS and Linux
winget install Tessariq.Taskrail       # Windows
taskrail --version
```

From source (needs Go `1.26`):

```sh
go install github.com/tessariq/taskrail/cmd/taskrail@latest
```

A plain `go build`/`go install` reports version `0.0.0-dev`; release builds inject
it with `-ldflags "-X main.version=vX.Y.Z"`. Tagged `v*` releases are published
automatically with [GoReleaser](https://goreleaser.com) for Linux/macOS/Windows on
`amd64`/`arm64`. Shell completion ships via Cobra —
`source <(taskrail completion bash)`, and `taskrail completion --help` for zsh and
fish.

## What a Task Looks Like

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

Task files are the durable ledger. `planning/STATE.md` is the *generated*
projection of them — active spec, current task, status summary, blockers, next
action, latest verification result. Never hand-edit it or a status field; the
`taskrail` commands own those writes. Repository-wide human context belongs in
`planning/NOTES.md`, a human-owned sidecar the CLI creates once and never
rewrites.

Create and steer tasks through the CLI rather than editing frontmatter:

```sh
taskrail task new --title "Add machine envelope" --area uniform-agent-machine-results
taskrail task rename T-007 --slug add-slug          # re-slug id + filename atomically
taskrail task repoint T-007 --area some-anchor      # move it onto another spec area
taskrail task dependency add T-010-api T-009-model  # one reviewed edge
```

## Repository Layout

```text
.
├── AGENTS.md          # guidance for coding agents
├── cmd/taskrail/      # CLI entry point
├── internal/          # core packages
├── planning/          # task ledger, generated STATE.md, optional human NOTES.md
└── specs/             # versioned, normative product specs
```

## Development

[mise](https://mise.jdx.dev) provisions the pinned toolchain (Go, `task`,
`lefthook`) from `mise.toml` — optional; direct `go` commands and the
`Taskfile.yml` targets work without it:

```sh
mise run setup   # provision, build taskrail onto PATH, wire the opt-in git hooks
go build ./cmd/taskrail && go test ./...
```

CI (`.github/workflows/ci.yml`) is the authoritative gate and runs the build/test
matrix over Linux, Windows, and macOS. Optional
[lefthook](https://github.com/evilmartians/lefthook) hooks mirror it locally
(`task hooks:install`), with `pre-push` on the shorter `task test:short` lane;
`pre-commit` also runs the agent-identity guard, and `commit-msg`/`pre-push`
refuse agent-attribution trailers and session links. Do not bypass them with
`--no-verify`. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the PR checklist and the AI-assisted
contribution policy, and
[AGENTS.md](AGENTS.md#toolchain-and-environment) for the binary-freshness
contract that matters when hacking on Taskrail itself.

## Status

Taskrail is an in-progress open-source project. The current release is `v0.4.0`;
the active development specification is `v0.5.0`.

- `v0.1.0` — the repository contract: deterministic task progression, the authoritative `STATE.md`, verification as a first-class concept.
- `v0.2.0` — adoption in existing repositories: `retrofit`, LLM-free `import`, opt-in agent skills, non-destructive `init`, `STATE.md` repair.
- `v0.3.0` — read-only insight (`status`, `stats`, `coverage`), the `spec` family, `unblock`, WinGet install.
- `v0.4.0` — active-spec selection and authoring, slugged creation with atomic rename/repoint, mechanical spec/gap review, version-skew detection.
- `v0.5.0` *(in development)* — uniform agent results, lifecycle-complete skills, human-owned repository notes, configurable review, safe review publication, a bounded external-process loop.

Roadmap beyond that is tracked in [`specs/README.md`](specs/README.md). The
versioned specs in `specs/` are the normative source of truth for release scope
and behavior.

## Read Next

- [`docs/commands.md`](docs/commands.md) — command reference: write effects, the JSON envelope, selection, locks and recovery, slugs, repoint, imports
- [`docs/loop.md`](docs/loop.md) — bounded unattended execution
- [`docs/migration.md`](docs/migration.md) — layout upgrades
- [`AGENTS.md`](AGENTS.md) — guidance for coding agents
- [`CHANGELOG.md`](CHANGELOG.md)

## License

Apache-2.0. See [LICENSE](LICENSE).
