# Skills Overview

Skill catalog for deterministic tracked-work execution in Taskrail.

## One Packaged Set

Taskrail ships a single, repo-agnostic skill set. There is no dogfooding-vs-shipped
split: this repository adopts the packaged skills like any adopter (T-055).

- **Source of truth:** `internal/taskrail/skills/`, embedded in the binary and
  installed by `taskrail init --with-skills` (T-030).
- **Committed copies (zero-setup clone):** `.agents/skills/` and `.claude/skills/`
  are kept in the tree so cloning this repo needs no install step.
- **Parity check:** `task check:skills` (Go test `TestCommittedSkillsMatchPackage`)
  asserts the committed copies are byte-identical to the embedded `--with-skills`
  output. It replaces the retired three-way `check-skill-mirrors.sh` diff and runs
  in CI and lefthook. Regenerate committed copies with `task skills:regen` after
  editing the package.
- Productization decisions live in `docs/workflow/skills-productization.md`.

## Packaged Skills

Tracked-work skills (create tasks with `${TASKRAIL:-taskrail} task new`):

- `autonomous-backlog`
- `autonomous-task`
- `autonomous-verify`
- `autonomous-recovery` — routes every correction through `taskrail repair`, never
  hand-editing authoritative state (shipped in T-054).
- `autonomous-manual-test` — its `planning/artifacts/manual-test/` artifacts stay
  ephemeral and gitignored, not a product invariant (shipped in T-081).
- `taskrail-repair`
- `taskrail-spec` — inspect and author specs, anchoring tracked work to real
  `spec_ref` headings via the `spec` command family (shipped in T-064).

Onboarding skills (create tasks with `${TASKRAIL:-taskrail} import --apply`):

- `taskrail-import` — notes/draft -> spec/task import.
- `taskrail-retrofit` — guided retrofit of an existing repository into a Taskrail
  layout.

## Configurable Entry Point

Skills invoke the binary through `${TASKRAIL:-taskrail}` (T-051), never
`go run ./cmd/taskrail`. In this repository the bare `taskrail` fallback is made
correct by building the working-tree binary onto the mise PATH — run
`mise run setup` (T-074). See `AGENTS.md` for the staleness trap this avoids.

## Required Behavior

- all skills invoke the binary via `${TASKRAIL:-taskrail}` and never `go run`
- implementation skills must keep changes scoped to one selected task
- verification skills must point to concrete artifact paths
- committed copies must stay byte-identical to the embedded package (parity check)
