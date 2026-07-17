---
name: taskrail-gap
description: Review structural gap signals and add agent semantic gap judgement over the active spec, LLM-free in the binary
---

# taskrail-gap

Find likely missing work — verification tasks, dependency fixes, decomposition, edge
cases, cleanup, rollout — over the active spec and task graph. The split below is the
whole point: the binary supplies the structural half deterministically, you supply the
semantic half. This composes the shipped `coverage --gaps` primitive (T-100) and adds
no new binary surface.

Requires the installed `taskrail` binary on `PATH`. Run it from the managed
repository's root.

## The mechanical-vs-semantic split

- **Structural (binary).** `coverage --gaps` emits mechanical candidates over covered
  active-spec areas — `missing-verification`, `dependency-anomaly`,
  `under-decomposed-area` — from counts and graph edges only. No inference, no model
  call. These are inspectable and reproducible.
- **Semantic (agent).** Deep gap inference is deliberately excluded from the binary
  (`specs/v0.4.0.md#recommendation-about-llm-support`). That judgement — which edge
  cases matter, what cleanup or rollout the spec implies, whether a structural signal
  is a real gap — is your job, done in this skill between deterministic binary steps.

Both halves stay advisory: they surface candidate tasks for a human to promote, never
auto-created state.

## When to use this vs the sibling skills

- **taskrail-gap** (this skill) reviews **covered** areas for *missing* work — the
  verification, decomposition, and edge-case gaps within areas that already have tasks.
- **taskrail-decompose** is **spec-driven** for **uncovered** areas: it fills coverage
  gaps against spec anchors that have no linked task yet.
- **taskrail-import** turns arbitrary external notes into tasks and may propose new
  spec sections.
- **taskrail-spec** inspects and authors specs and anchors single tasks.

## Flow

1. **Get structural candidates.** Run `${TASKRAIL:-taskrail} coverage --gaps --json`.
   Each entry in `signals` is a mechanical candidate: `kind`, the area `anchor`, and a
   human-readable `detail`. `coverage` is read-only and writes nothing. To focus a
   single area, compose with area scoping:
   `${TASKRAIL:-taskrail} coverage --gaps --area <anchor> --json` — it uses the same
   area resolution and rejection rules as normal coverage scoping. Do not pass `--min`;
   gap analysis does not gate.
2. **Review semantically.** For each structural signal, decide whether it is a real
   gap and why. Then go beyond the mechanical signals: read the active spec area and
   its linked tasks and name the gaps the binary cannot detect — missing edge-case
   handling, absent rollout/migration/cleanup work, verification that exists but is
   too shallow, dependencies that are semantically wrong even when the graph is
   well-formed. Ground every candidate in the spec text and existing tasks.
3. **Propose candidate tasks.** Present the surviving gaps as a concise list — title,
   the `spec_ref` anchor each belongs to, and a one-line rationale. This is a
   recommendation for a human, not state: the skill never creates tasks on its own.
4. **Human promotes chosen ones.** A human reviews the list and promotes the ones they
   accept — a single task with `${TASKRAIL:-taskrail} task new`, or a reviewed batch
   through `taskrail-decompose`'s `${TASKRAIL:-taskrail} import --apply` draft path.
   The binary is the only writer, and only under that explicit human-invoked apply.
5. **Validate.** After any promotion, run `${TASKRAIL:-taskrail} validate` and confirm
   the state is valid.

## Rules

- never hand-edit `planning/STATE.md` frontmatter or task status fields
- `coverage --gaps` is read-only and advisory; it never writes state or gates `validate`
- proposals are candidates for human review, never auto-created tasks; promote only
  through `${TASKRAIL:-taskrail} task new` or `import --apply`, never by hand-authoring
  task markdown
- do not pass `--min` with `--gaps`; gap analysis does not gate and rejects the combo
- deep semantic gap inference stays out of the binary by design; it lives here
