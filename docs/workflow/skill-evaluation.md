# Skill Evaluation

Maintainer contract for evaluating Taskrail's packaged Agent Skills.

## Boundaries

- Required CI remains credential-free and deterministic.
- Behavioral runs are maintainer-started release evidence, not product runtime.
- Eval definitions live outside `internal/taskrail/skills/` and are not installed
  by `taskrail init --with-skills`.
- Taskrail does not choose a provider, calculate model cost, persist conversations,
  or interpret model output inside the binary.
- Eval output never changes task status, verification truth, specs, packaged skill
  sources, committed mirrors, or Git history automatically.

## Required Contract Checks

Every packaged skill is checked mechanically for valid Agent Skills frontmatter,
resolvable references, real commands and flags, common-JSON use when consuming
results, canonical lifecycle order, provider independence, nested-resource
packaging, and byte parity across embedded and committed copies.

## Evaluation Registry

The checked registry lives at `internal/taskrail/testdata/skill-evals/v1/cases/`,
one strict `case.json` per case. Every shipped skill has committed and local
cases; local cases include exact stale logical-state decoy bytes and a Git
provenance sentinel. The registry validator rejects missing skill/mode coverage,
duplicate case IDs, path/name disagreement, malformed strict JSON, incorrect
v0.4.0 baseline classification, and incomplete local fixtures.

Case and registry fixture digests use the domain-separated tree framing in
`specs/v0.5.0.md#maintainer-skill-release-evaluations`. These assets define the
complete deterministic input set. They contain no provider runner, credentials,
transcripts, raw evidence, or installed skill content. T-307 owns caller-adapter
execution and report construction.

## Manual Behavioral Run

1. Freeze the case set, deterministic assertions, candidate skill bytes, and prior
   released baseline bytes by SHA-256.
2. Run realistic positive, negative, recovery, and boundary cases through a
   caller-owned agent adapter with candidate and baseline in isolated sandboxes.
3. Preserve raw outcomes, missing runs, adapter/model identity, timing and usage
   when available, and deterministic repository grades.
4. Review paired outputs without treating model judgement as a mechanical fact.
5. Let an agent propose candidate patches in an isolated workspace if failures
   reveal a general skill problem. Do not let it edit fixtures or shipped sources.
6. Have a human select and apply any revision, rerun required checks, then run an
   untouched final case set and retain the release report.

An absent credential, timeout, incomplete pair, or missing grade is explicit
incomplete evidence, never a passing evaluation.

The maintainer harness is `SkillEvalRunner` in `internal/taskrail`. A caller
supplies its provider adapter, the status-reported artifacts root, fixed
candidate and baseline evidence bindings, and the human comparisons. The runner
invokes every required arm once, accepts an adapter error as a missing arm,
rejects unsafe or empty raw evidence, and returns an unpersisted schema-v1 safe
summary. `RenderSkillEvalReport` produces canonical JSON for human review; it
never writes a durable review, alters skills or fixtures, or applies a proposal.

## Waived Evidence

A report normally has a null `waiver`. A named release maintainer may report
`outcome: "waived"` only when credential-free deterministic checks pass and the
only incomplete evidence is exactly covered by a non-null waiver. The waiver
records its approver, reason, unavailable capability, affected skills and cases,
residual risk, sorted compensating evidence, and follow-up issue or release
target. It cannot cover failed checks or cases, and it does not establish the
approver's release authority; repository governance does that outside the report.
Its deterministic-check evidence must also name each sorted credential-free gate:
`command`, `cross-platform`, `lifecycle`, `machine-api`, `parity`, and `security`.

`fail` and `incomplete` remain release blockers. A release using `waived` must
disclose the waiver, residual risk, and follow-up in its release notes. Raw
provider output remains under ignored `planning/artifacts/skill-evals/` and is
never included in the committed report or release notes.

## Research Basis

The paired baseline, assertion, human-review, and proposal-only iteration method
adapts ideas from Anthropic's
[`skill-creator`](https://github.com/anthropics/skills/tree/main/skills/skill-creator).
The separation between deterministic contract checks and periodic behavioral
cases follows the useful part of
[`mthines/agent-skills` evals](https://github.com/mthines/agent-skills/blob/main/scripts/eval/README.md).
Taskrail deliberately does not vendor either provider-specific runner.

Archon's prompt/workflow design also informed the cases: stop on material
ambiguity, inspect existing primitives before proposing new ones, isolate review
lenses, use fresh iteration contexts when available, and keep deterministic exit
checks separate from agent claims. Taskrail does not adopt Archon's YAML workflow,
provider, worktree, or PR orchestration surfaces.
