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
- Maintainer evaluation boundaries live in `docs/workflow/skill-evaluation.md`.

## Packaged Skills

Tracked-work and spec skills (each bullet notes how it creates tasks, if at all):

- `autonomous-backlog`
- `autonomous-task`
- `autonomous-verify`
- `autonomous-recovery` — routes every correction through `taskrail repair`, never
  hand-editing authoritative state (shipped in T-054).
- `autonomous-manual-test` — its `planning/artifacts/manual-test/` artifacts stay
  ephemeral and gitignored, not a product invariant (shipped in T-081).
- `taskrail-loop` — interactively configures, previews, confirms, and supervises
  one bounded provider-neutral `loop` invocation. It reports coordinator and
  caller-owned adapter evidence but never selects work or gains lifecycle, Git,
  integration, delivery, or recovery-write authority (shipped in T-337).
- `taskrail-repair`
- `taskrail-spec` — inspect and author specs, anchoring tracked work to real
  `spec_ref` headings via the `spec` command family (shipped in T-064).
- `taskrail-spec-review` — stage four independent advisory post-spec lenses and
  publish their human-dispositioned digest-bound bundle before decomposition
  (shipped in T-162).
- `taskrail-decompose` — author strict spec-anchored ImportDraft v2 bodies, run at
  most two fresh-context adversarial passes, publish an immutable manifest-bound
  bundle, and apply its exact digests (shipped in T-098; reviewed flow in T-304).
- `taskrail-sdd-handoff` — turn an operator-selected, already reviewed OpenSpec or
  Spec Kit artifact set into an advisory, content-based handoff to existing
  Taskrail spec, import, and decomposition flows. It stops on ambiguity and never
  applies or synchronizes source artifacts (shipped in T-202).
- `taskrail-gap` — review covered active-spec areas for missing work: run
  `coverage --gaps --json` for structural candidates, add agent semantic judgement,
  and propose tasks a human promotes via `task new` / `import --apply` (shipped in
  T-101). This is a deliberate split: the binary's `coverage --gaps` stays
  **mechanical** — count, graph, and state signals only, **never semantic** "this
   needs a test" inference — and the skill supplies the semantic half. Structural
   signals are candidates to promote, not violations; see the "Coverage vs gap
   analysis" section in `README.md` for the full boundary.
- `taskrail-task-review` — inspect one existing task against its referenced spec,
  related tasks, and dependencies; stage one strict digest-bound advisory review
  and publish it through `review publish --type task` without directly changing
  task state (shipped in T-216).
- `taskrail-workflow-adversarial` — run one bounded post-implementation review in
  an isolated sandbox and publish only the strict Git/spec/product-bound report
  and Taskrail-derived serial memory (shipped in T-306).

Onboarding skills (create tasks with `${TASKRAIL:-taskrail} import --apply`):

- `taskrail-import` — notes/draft -> compatibility ImportDraft v1 -> scaffolded,
  implicitly held spec/task import; legacy v1 task bodies are ignored on apply.
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
- skills use `--json` for consumed IDs, paths, warnings, eligibility, previews,
  lifecycle outcomes, and failures; exact content and emitted prompt bytes remain
  explicit text exceptions
- committed copies must stay byte-identical to the embedded package (parity check)

## Active v0.5 Additions

The active v0.5 roadmap adds `taskrail-spec-review`, `taskrail-task-review`,
`taskrail-workflow-adversarial`, and `taskrail-sdd-handoff`. Existing packaged
skills consume structured command facts through the common machine-result
envelope. `spec show` bodies and `--emit-prompt` output intentionally remain exact
text because those bytes are workflow input rather than a report to interpret.

Behavioral eval definitions for the packaged set are maintainer-only test assets.
They are deliberately outside `internal/taskrail/skills/`, so neither
`init --with-skills` nor the committed parity copies contain eval cases, runners,
credentials, transcripts, or benchmark output.
