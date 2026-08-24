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
- [Layout Upgrades](#layout-upgrades)
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
`command`, `warnings`, and exactly one of `result` or `error`. This is the one-time
v0.5 break from pre-v0.5 bare result objects; consumers must reject unsupported
versions rather than decode an inherited shape optimistically. Text output stays
human-oriented and unchanged. See the
[envelope contract](docs/commands.md#machine-output-envelope).

Idle `next` selection is anchored to the active spec: it considers only `todo`
tasks whose `spec_ref` points at the active spec. When only older-spec work is
runnable, `next` reports no eligible task and lists the skipped tasks under
`warnings`; an already-active off-spec task is still returned so you can continue
or resolve it. Recover older work explicitly with `start <id>` or
`next --include-off-spec`, or move it onto the active spec with
[`task repoint`](docs/commands.md#re-pointing-a-task-onto-another-spec-area).
Details: [docs/commands.md](docs/commands.md#next-task-selection-and-the-active-spec).

**Beyond the core loop**

- **Adopt an existing repo** — `init` and `retrofit` scaffold `specs/` + `planning/` non-destructively; `import` turns rough notes into spec/task drafts without an LLM; `repair` reconciles mechanical `STATE.md` drift.
- **See where work stands** — `status`, `stats`, and `coverage` report a live snapshot, aggregate metrics, and advisory spec-linkage, all read-only. `status` also breaks down open work (`todo`/`in_progress`/`blocked`) by how much targets the active spec versus points away from it, listing the away tasks and their `spec_ref`; the away set matches the active-spec filter `next` uses for idle selection.
- **Author and steer specs** — the `spec` family (`list`, `show`, `add`, `activate`, `diff`) inspects and evolves versioned specs; `spec diff` previews the mechanical area-set delta before activation.
- **Inspect workflow evidence** — `prompt list` reports the versioned embedded catalog and committed replacement source, `prompt show <id>` prints the resolved template or `--builtin` bytes, `loop --dry-run --json` publishes the one allowed task and frozen implementation prompt without launching it, and `review show <logical-path>` returns exact durable-review bytes through active storage without creating files.
- **Publish review evidence** — `review publish --type task` validates one ignored task proposal against exact task and spec snapshots; `--type spec` validates an approved ignored four-lens bundle and exact selected spec snapshot; and `--type decomposition` validates a complete reviewed draft/trace bundle against exact selected-spec and published spec-review snapshots. Each preserves selected JSON bytes in one absent durable session directory; use `--dry-run` to inspect the same candidate without writing it.
- **Draft missing work** — the optional `taskrail-decompose` and `taskrail-gap` skills turn uncovered areas and structural gap signals into reviewable proposals; only an explicit `task new` or `import --apply` writes tracked tasks.
- **Handle the messy parts** — `task show <id>` reads one exact task's persisted Markdown through the active storage context. `block`/`unblock` park and resume work; `task release <id> --reason "..."` deliberately returns interrupted active work to `todo` without fabricating blocker or cancellation history; `task new` scaffolds a task, `task rename` re-slugs it, `task repoint` moves its `spec_ref`, and `task dependency add|remove` changes one reviewed dependency edge. When a crashed writer leaves the repository mutation lock behind, `lock status` inspects it read-only and `lock clear` removes exactly the observed stale lock — never automatically, and never while its owner is provably alive on this host. When a crashed durable transaction leaves a recovery fence behind, `recover <transaction-id>` previews and — with `--apply` — performs the one mechanically safe action (restore originals, accept the validated candidate, or clear the fence).

Run `taskrail --help`, or `taskrail <command> --help`, for the full command list and every flag.

### Command effects

Taskrail commands intentionally use different write conventions based on risk:

| Class | Current examples | Effect |
|---|---|---|
| Read-only | `validate`, `status`, `stats`, `coverage`, `task show`, `spec list/show/diff`, `prompt list/show`, `loop --dry-run`, `review show`, `lock status` | Inspect only; never rewrite tracked planning state. |
| Mode-dependent initialization | `init` | Fresh, unmarked-standard, and current-layout adoption/repair paths may write immediately; detected migration or retrofit paths preview unless `--apply` is supplied. Fresh/adopted writes, including `--with-skills`, publish as one locked normal transaction. A repository at layout 1 reports the read-only layout 2 upgrade preview instead: the flagless invocation writes nothing, and `--apply` requires `--confirm-quiescent` plus the note and skill decisions the preview names. |
| Preview by default | `retrofit`, `repair` | Report a candidate; `--apply` is the write opt-in. Retrofit apply publishes its complete scaffold under the repository lock, while its preview rechecks inputs without creating lock or transaction artifacts. |
| Apply with preview option | `task rename`, `task repoint`, `task release`, `task dependency add/remove` | Write by default; `--dry-run` validates the candidate first. |
| Lifecycle/state writers | `next`, `start`, `complete`, `block`, `unblock`, `task release`, `verify`, `spec activate`, `task new` | Rewrite `STATE.md` and sometimes task files; inspect `git status` afterward. |
| Operator lock recovery | `lock clear <lock-id> --expect-sha256 <digest>` | Removes only the unchanged mutation lock observed via `lock status`; refuses a provably live same-host owner and never touches retained transaction data. Never rewrites tracked planning state. |
| Operator transaction recovery | `recover <transaction-id> [--take-over-lock <lock-id> --expect-sha256 <digest>] [--apply]` | Previews the single safe action a retained durable transaction derives (restore-original, accept-candidate, clear-fence); `--apply` performs exactly that action. A held lock requires the paired, exact observed takeover operands; a live same-host owner still refuses. |
| Reviewed import writer | `import --apply <draft>` | Validates an external draft and writes its bounded task/spec/state set. |
| Review publisher | `review publish --type task\|spec\|decomposition` | Validates an ignored proposal and exact reviewed subjects; `--dry-run` is read-only, while apply takes the writer lock and creates one absent review session directory without changing task lifecycle or planning state. |

`next` is not a read-only selection probe: it persists `next_action` and
`updated_at`. Use `status` when you need the same next-task computation without a
tracked write.

`next`, `start`, `complete`, `block`, `unblock`, `task release`, and `verify` take the repository
mutation lock and publish their exact state/task write set (plus verification
artifacts and any `--create-followup` task) transactionally. So do the task
mutation writers — `task new`, `task rename`, `task repoint`, and
`task dependency add/remove`: rename publishes its coupled move by filesystem
operations (no `git mv` staging), and each writer publishes only the task and
state files it declares. A concurrent writer refuses with `lock_held`; unselected
task bytes are never re-encoded.

Each successful `complete` also creates a fresh random lower-case 32-hex
`completion_id`, persists it on the completed task, and returns that exact value.
Completing again replaces the identity, and completion clears prior task-level
verification metadata without changing repository-level verification history.

All semantic command classes share one recovery admission fence: retained or
malformed transaction state beneath the canonical repository runtime root makes
readers and writers fail with `recovery_pending`, and writers do not begin.
`recover` and read-only `lock status` are the only fence-admitted operator
surfaces. See
[docs/commands.md](docs/commands.md#recovery-admission-fence).

### Coverage vs gap analysis

`coverage` answers **"is this spec area linked to any task?"**;
`coverage --gaps` answers **"does a *covered* area lack a verification/companion task, have
a dependency-graph anomaly, or look under-decomposed?"** Both are read-only and
advisory by default — they never write `STATE.md` or task files and never make
`validate` fail; `--gaps` opts into gating only through `--fail-on <category>`.
The hard limit: `--gaps` is **mechanical only** — its signals are **candidates, not violations**: false positives are expected, and each is something to inspect
and promote into a real task, never a semantic "this needs a test" rule. For the
semantic half use the `taskrail-gap` skill. Details:
[docs/commands.md](docs/commands.md#coverage-vs-gap-analysis).

### Review stages

Taskrail keeps mechanically testable state (`validate`, `coverage`) separate from
agent or human semantic review (`verify` records evidence against one task).
Semantic findings never become `validate` violations automatically; humans adopt
accepted changes through the bounded task/spec/import commands. The active v0.5
roadmap adds distinct advisory review stages — see
[docs/commands.md](docs/commands.md#review-stages).

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
flag, it completes spec versions, real `<path>#<anchor>` values for
`--spec-ref`, and the active spec's bare anchors for `--area` flags — exactly
the anchors `validate` accepts.

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

Tasks may also carry paired `loop_policy: allow|hold` and `loop_reason: <reason>`
frontmatter. Omitting both means an implicit hold with reason
`implicit hold: loop policy is not set`; `taskrail validate` rejects incomplete
or malformed pairs. Lifecycle and task writers preserve explicit policy metadata,
and `STATE.md` does not duplicate it.

Use `taskrail task loop list [--json]` to inspect every task's effective loop
policy, held dependency closure, and unattended eligibility without changing
task files, `STATE.md`, or ordinary lifecycle selection.

Use `taskrail loop --dry-run --json` to inspect the highest-ranked explicitly
allowed task, its frozen implementation prompt, and delivery/review limits. Use
`--parallel <n>` to preview one ranked, dependency-ready clone frontier without
creating workspaces or launching workers; clone and delivery options are refused
when their effective width is one. With a width above one, execution runs that
frozen frontier in private `--no-local` shallow clones, replays valid worker
commits in selection order in a separate integration clone, and fast-forwards the
source branch only after aggregate validation. `--clone-depth full` opts out of
the shallow boundary; `--keep-workspaces` retains failed workspaces by default.
Local delivery is the default. `--delivery review --review-adapter <path>` sends
each provider-neutral JSON request directly to one caller-owned executable; it
does not embed credentials, provider APIs, or shell evaluation. The adapter may
wrap tools such as `gh` or `glab`, but owns their authentication and provider
semantics. Passing worker branches and changes can open concurrently; Taskrail
inspects, merges, and refreshes remaining changes in frozen frontier order. A
failed worker, adapter operation, check, or refresh is not retried, but other
independently valid changes remain eligible. A replacement implementation prompt
requires its exact template SHA-256 through `--allow-prompt-override-sha256`; dry
run never launches a child process.

Execution keeps child output streaming, so use `--result-file <external-path>`
to receive its one terminal schema-1 envelope. The target must be absent in an
existing, non-symlinked directory outside the worktree and Git metadata;
Taskrail rechecks that directory and creates the file without replacement after
postflight.

Use the packaged `taskrail-loop` skill when an operator needs one interactive,
provider-neutral loop run. It elicits unresolved caller-owned choices, explains
the structured dry-run and requires confirmation, then supervises the one
coordinator invocation through its external result file. The skill reports
worker, integration, delivery, and optional caller-owned CI evidence, but never
selects work, mutates Git or planning state, or substitutes for Taskrail's
coordinator or review adapter.

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

Added areas are the ones a migration must decompose into tasks; removed areas
are the ones whose existing tasks become orphaned drift; rename candidates are
best-effort, labeled for you to verify, never asserted as fact. It is
side-effect-free, and `--json` mirrors the output with structured
`added`/`removed`/`renamed` lists.

### The slug-in-id invariant

A task's `id` and its filename are two encodings of one identifier: `validate`
enforces `filename == "<id>.md"`, so a slugged filename requires a slugged id.
`task new` produces that pairing directly — `--title "X"` derives a slug and
writes `T-<n>-x-slug` with a matching `T-<n>-x-slug.md`, `--slug` overrides the
slug source, and passing neither keeps the bare `T-<n>` / `T-<n>.md` form. Every
case passes `validate` with no follow-up edit. Because id and filename move
together, you cannot rename a file for readability on its own — a bare
`git mv T-<n>.md T-<n>-add-slug.md` leaves the frontmatter `id:` bare, and the
next `validate` fails with `task <id> filename must be <id>.md`. The fix is
`task rename`, which re-slugs atomically (id, filename, heading, inbound
dependency refs, `STATE.md`):

```sh
taskrail task rename T-<n> --slug add-slug     # or --title "Add slug"; --dry-run previews
```

`task rename` re-encodes the identifier only — it never rewrites the `title:`
frontmatter field, and there is no `task retitle` command in this version, so
`rename --title "New Title"` derives a new slug and leaves the title unchanged;
to retitle, edit the `title:` field directly. Edge cases — accented
transliteration, the ~50-character slug cap, symmetric slug stripping — are
covered in
[docs/commands.md](docs/commands.md#the-slug-in-id-invariant).

### Re-pointing a task onto another spec area

After `spec activate`, open tasks still pointing at the previous spec are off-spec:
`next` skips them, `status` lists them under the active-spec drift breakdown, and
`next --include-off-spec` recovers one to run where it is. To move an open task
*onto* the active spec instead, `task repoint` rewrites its `spec_ref` — the one
edit that would otherwise mean hand-editing frontmatter:

```sh
taskrail task repoint T-<n> --area status-active-spec-drift-breakdown  # active-spec anchor
taskrail task repoint T-<n> --spec-ref specs/v0.2.0.md#some-area       # explicit, cross-spec
```

`--area` resolves the anchor against the active spec exactly as `task new --area`
does, so an unknown anchor fails before any write; `--dry-run` previews the state
the repoint *would* leave behind. Repoint never touches id, slug, filename, title,
status, or dependencies; completed and cancelled tasks are rejected. Because it
re-projects `planning/STATE.md`, run `git status` afterwards. Details:
[docs/commands.md](docs/commands.md#re-pointing-a-task-onto-another-spec-area).

### Editing one dependency edge

Use exact full persisted task IDs to apply one accepted dependency-review change:

```sh
taskrail task dependency add T-010-api T-009-model --dry-run
taskrail task dependency add T-010-api T-009-model
taskrail task dependency remove T-010-api T-009-model
```

Add appends without reordering and rejects missing, self, duplicate, cancelled,
or cyclic edges; remove rejects an absent edge. Both preserve all other task
bytes, transactionally publish the task with a reprojected `STATE.md`, and
support the common `--json` envelope.

Bootstrap drafts from rough notes without any LLM — preview first, then apply:

```sh
taskrail import notes.md --to tasks                # preview the structural task drafts
taskrail import notes.md --to tasks --emit-prompt  # print an agent prompt for a richer draft
taskrail import --apply draft.json                 # validate an agent draft and write real files
```

An apply that fails during writing exits non-zero and still reports what it wrote
or may have touched. Review those paths before retrying — a failed spec write may
leave an empty or truncated file, and re-applying the same draft creates any
already-written tasks a second time under new ids. Details:
[docs/commands.md](docs/commands.md#import-drafts-and-partial-writes).

Typical flow:

1. Write a goal as a Markdown task inside `planning/tasks/`.
2. `validate` the repository.
3. `next` to select deterministically, then `start`.
4. `complete` the implementation.
5. `verify` to record the outcome and leave artifacts — opening follow-up tasks as needed.

## Layout Upgrades

A repository whose layout marker sits at layout 1 has a read-only layout 2
upgrade preview: plain `taskrail init` reports every operator decision the
upgrade resolves before anything can apply — the complete candidate paths
(marker, schema-2 state, preserved task files, notes sidecar), committed
storage, the default broad review-round maximum, decoded continuation notes
with their applicable `extract`/`drop` choices, and each installed skill's
classification (parity mirrors stay marker-free; stamped copies normalize
through a forced refresh). The preview writes nothing, and a blocking state —
an `AUTONOMY.tsv` legacy entry at the configured planning path, an unsafe
notes destination, or a divergent or conflicting skill copy — refuses with
actionable guidance instead.

Applying the upgrade is gated and durable: `taskrail init --apply` requires
`--confirm-quiescent` (your assertion that every older Taskrail process able
to touch this repository or its linked-worktree storage has stopped), exactly
one of `--extract-continuation-notes` or `--drop-continuation-notes` when
decoded notes exist and neither when they do not, and the combined
`--with-skills --force` whenever stamped skill copies require normalization.
A fully gated apply publishes the exact previewed candidate through one
recoverable transaction: the marker is fenced as layout 2 with a
`migration_fence` transaction id before any task, state, note, or skill byte
changes, the complete candidate publishes and post-validates, and the strict
final marker replaces the fence as the transaction's last operation. A handled
failure rolls every candidate-written byte back before the original marker;
an interruption leaves the fence plus the retained transaction, every other
command refuses (`recovery_pending` with the transaction, or
`migration_in_progress` when only the fenced marker remains), and
`taskrail recover <transaction-id>` derives the single safe restore, accept,
or clear action. Older binaries refuse layout 2 through the command-wide
compatibility guard, and downgrade is complete Git reversion of the upgrade —
never hand-editing the marker. An explicit
`init --with-skills` request on a layout 1 repository is served by the current
layout, so skill installation keeps working independently of the upgrade.

## What a Verification Leaves Behind

Every verification creates a fresh lower-case 32-hex identity, records its direct
predecessor when one exists, and writes repo-local evidence under
`planning/artifacts/verify/<task-id>/<timestamp>-<verification-id>/`:

```text
planning/
  STATE.md                         # generated current execution projection
  NOTES.md                         # optional human-owned repository context
  tasks/
    T-001.md                       # task with frontmatter schema
  artifacts/
    verify/
      T-001/
        20260619T113646Z-0123456789abcdef0123456789abcdef/
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

`planning/STATE.md` is the authoritative current execution projection. It carries the active spec, current task, status summary, blockers, the next action, and the latest verification result and identity tuple, plus pointers to relevant artifacts. It is not a per-task or per-session log: keep durable task context in task `## Implementation Notes`, blocker reasons, portable verification summaries/reports, or follow-up tasks. Repository-wide human context lives in `planning/NOTES.md`, a human-owned sidecar `init` and `retrofit --apply` create as a short commented template when that path is absent and never rewrite afterwards; agents may read it but edit it only when explicitly asked. Do not hand-edit machine-managed state fields or append continuation prose; let the `taskrail` transitions update the file.

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
`task check:skills`. Installed skills record the Taskrail version that wrote them
in `metadata.taskrail_version`, so version skew is detectable and advisory —
the details, including why you must not run `init --with-skills --force` in this
repository, live in
[docs/workflow/skills-productization.md](docs/workflow/skills-productization.md).

## Development

[mise](https://mise.jdx.dev) provisions the pinned toolchain (Go, `task`,
`lefthook`) from `mise.toml` — optional; direct `go` commands and the
`Taskfile.yml` targets work without it:

```sh
mise run setup   # provision, build taskrail onto PATH, wire the opt-in git hooks
go build ./cmd/taskrail && go test ./...
```

CI (`.github/workflows/ci.yml`) is the authoritative gate: it provisions the
same toolchain via [`jdx/mise-action`](https://github.com/jdx/mise-action) and
runs the build/test matrix over Linux, Windows, and macOS. Optional
[lefthook](https://github.com/evilmartians/lefthook) git hooks mirror CI locally
(`task hooks:install`, or `go install
github.com/evilmartians/lefthook@v1.13.6`; `pre-commit` runs `gofmt`/`go vet`/
`validate` plus the skill-parity and binary-freshness guards). Do not bypass
them with `--no-verify`.

The binary-freshness guards and the mise/PATH wiring they rely on matter when
hacking on Taskrail itself — see
[AGENTS.md → Toolchain And Environment](AGENTS.md#toolchain-and-environment)
for the full contract. See [CONTRIBUTING.md](CONTRIBUTING.md) for the PR
checklist, the AI-assisted contribution policy, and tracked-work rules.

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
- [`docs/commands.md`](docs/commands.md) — command deep-dive reference (envelope, gaps, slugs, repoint, import)
- [`specs/README.md`](specs/README.md) — spec reading order and versioning
- [`planning/STATE.md`](planning/STATE.md) — live execution state
- [`AGENTS.md`](AGENTS.md) — guidance for coding agents
- [`CHANGELOG.md`](CHANGELOG.md)

The versioned specs in `specs/` remain the normative source of truth for release scope and behavior.
