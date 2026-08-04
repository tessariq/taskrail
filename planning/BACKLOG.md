# Backlog

Durable home for post-review ideas that were deliberately deferred out of the
current tracked version. These are candidates, not committed work: promote one
to a task (`taskrail task new`) against the version whose spec adopts it.

## Adopted into v0.4.0

- **`taskrail spec diff <v1> <v2>`** — mechanical anchor-set diff between two
  spec versions (areas added / removed / renamed). Adopted into `specs/v0.4.0.md`
  (Spec Version Diff area) during the v0.4.0 task review and tracked as **T-113**.

- **Version skew detection** — materialized skills never refresh on a binary
  upgrade, and the `layout_version` refusal only fires in `init`. Adopted into
  `specs/v0.4.0.md` (Version Skew Detection area) after the T-119 review and
  tracked as **T-120**/**T-121** (stale-skill warning and its version marker),
  **T-122** (layout guard in every command), and **T-123** (the contributor-side
  binary resolution trap that surfaced it).

## Deferred Design

- **Physical task archival (`taskrail task archive`)** — deferred from `v0.5.0`
  because archived files must remain one logical ledger with live tasks: IDs and
  numeric prefixes cannot be reused, completed dependencies must still resolve,
  and coverage, verification, state counts, stats, imports, rename/repoint,
  autonomy policy, and loop postflight must retain delivered history. Prefer an
  explicit post-verification archive over moving a file during `complete`. A
  future spec should define storage class, dry-run and Git-move atomicity,
  read/report scope, migration, and recovery together behind a new layout version;
  an older binary must never silently ignore archived tasks.

## Ideas

Unversioned extension ideas from a competitive scan against OpenSpec
(Fission-AI) and GitHub Spec Kit — the two adjacent spec-driven-development
tools. Same family as Taskrail (deterministic CLI + markdown artifacts, LLM
reasoning delegated to the agent), so their mature surfaces are a useful parity
checklist. These are candidates, not committed work; each stays LLM-free in the
binary and agent-assisted per Taskrail's design. Promote one to a task against
the version whose spec adopts it.

- **Change-proposal flow (`taskrail change`)** — the largest structural gap vs
  OpenSpec. Today Taskrail moves between whole versioned spec snapshots via
  `spec activate`; OpenSpec's `propose → apply → archive` loop lets a reviewed
  *change* carry a spec delta that merges into the source spec on archive. A
  Taskrail `change new / change archive` pair would model iterative brownfield
  edits (bug fix, small feature) as first-class deltas without minting a whole
  new spec version, then fold the accepted delta back into the active spec.
  Keeps the "living spec" promise measurable through existing `coverage`. Big
  design decision — schema-touching; ask before building.

- **Consistency / clarify gate (`taskrail analyze`)** — Spec Kit ships
  `/clarify` (surface underspecified areas before planning) and `/analyze`
  (cross-artifact consistency check across spec/plan/tasks). Taskrail already
  computes orphan and coverage drift; a read-only `analyze` could extend that to
  flag tasks whose acceptance criteria are thin, spec anchors with no
  decomposition, or dependency cycles — a single "is this planning coherent?"
  report. Read-only, deterministic, fits the `status`/`stats`/`coverage`
  read-only family.

- **Interactive dashboard (`taskrail view`)** — OpenSpec ships a terminal
  dashboard; Spec Kit leans on the agent. A TUI (or a rich `status --watch`)
  over the existing snapshot — active spec, task board by status, blockers,
  coverage — would be a read-only projection of state already computed, no new
  writes. Ergonomics only; low schema risk.

- **Project-principles doc (`taskrail constitution` / `principles`)** — Spec
  Kit's `/constitution` pins durable project constraints the agent must honor
  across every feature. Taskrail could adopt a `planning/PRINCIPLES.md` (or a
  spec preamble) that `init --with-skills` seeds and the packaged skills read,
  giving adopters a stable place for non-negotiables. Doc + skill wiring, no CLI
  state changes.

- **Broaden the packaged-skill surface beyond Claude** — OpenSpec targets 20+
  assistants, Spec Kit 30+. Taskrail ships `.claude/` and `.agents/` copies of
  one packaged set. Because the skills are static provider-agnostic text
  invoking `${TASKRAIL:-taskrail}`, widening `init --with-skills` to emit
  Cursor / Copilot / Codex layouts (or a generic `AGENTS.md`-style drop) is
  mostly a packaging-and-parity-check extension, not new runtime behavior. Grows
  the parity check surface — confirm scope before adding install targets.

- **`taskrail spec migrate` (builds on shipped migration primitives)** — a guided
  migration that, on `spec activate`, walks the
  added areas (offer `task new` for each) and the vanished areas (flag orphaned
  tasks for re-point or cancel). Turns the anchor-set delta into actionable
  tracked-work moves. The shipped `spec diff` (T-113) provides the delta, and the
  shipped `task repoint` (T-114) provides the re-point step.
