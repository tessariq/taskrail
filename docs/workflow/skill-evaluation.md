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

Each case has a strict executable `scenario`: the fixture source, a sandbox name
equal to its case ID, setup commands, and action command vectors. Commands are
only documented `taskrail` or `git` invocations. `fixture/seed.json` is a
validated concrete initialization recipe: it requires `git init`, selects the
documented committed or local `taskrail init --json` form, and local seeds carry
their decoy and provenance bytes. Its `oracle` maps every authored assertion
exactly once to a supported mechanical predicate over action facts. The supported
predicates are `command-exit-zero`, `taskrail-validation-pass`, and
`git-worktree-clean` (the named Git command exits zero and leaves the observed
worktree digest unchanged). An adapter returns structured facts containing exact
command argv/exit code, stdout/stderr, filesystem and Git before/after digests,
validation result, and storage paths, then writes the same canonical `facts.json`
receipt beneath its raw root. The runner rejects missing, extra, fabricated, or
receipt-mismatched facts and derives the deterministic grade only from predicate
evaluation; adapters do not supply assertion names or grades. Semantic claims
such as ambiguity handling, authority, or safe repair belong only in
`human_review_questions`. Case and registry fixture digests use
the domain-separated tree framing in
`specs/v0.5.0.md#maintainer-skill-release-evaluations`. These assets define the
complete deterministic input set. They contain no provider runner, credentials,
transcripts, raw evidence, or installed skill content. T-307 owns caller-adapter
execution and report construction.

## Manual Behavioral Run

1. Freeze the case set, deterministic assertions, candidate skill bytes, and prior
   released baseline bytes by SHA-256. Select the candidate executable from the
   clean attached tested HEAD and the baseline executable from the fixed v0.4.0
   commit on the same evaluation platform; record their digests.
2. Run every candidate and required baseline arm exactly once through a
   caller-owned agent adapter with candidate and baseline in isolated sandboxes.
   Execute every declared setup and action command, retain the stub or provider
   transcript as raw evidence, and record only actual command facts.
3. Render and retain the canonical sealed stage beside the producer-local raw
   evidence, then stop for a human worksheet. The stage contains no producer-local paths;
   resume reconstructs them from current input and refuses a changed receipt or
   raw tree. Do not render a final report or invent a comparison here.
4. Decode and resume from that exact staged evidence after human review. A completed paired
   case is `same`, `better`, or `worse`; a completed skill without a v0.4.0 arm is
   exactly `candidate-only`; missing or incomplete required evidence is
   `inconclusive`.
5. Let an agent propose candidate patches in an isolated workspace if failures
   reveal a general skill problem. Do not let it edit fixtures or shipped sources.
6. Have a human select and apply any revision, rerun required checks, then run an
   untouched final case set and retain the release report.

An absent credential, timeout, incomplete pair, or missing grade is explicit
incomplete evidence, never a passing evaluation.

The maintainer harness is `SkillEvalRunner` in `internal/taskrail`. A caller
supplies its provider adapter, the status-reported artifacts root, and fixed
candidate and baseline evidence bindings. Each adapter request receives the
case's resolved fixture directory, sandbox name, setup, and action vectors.
`Execute` invokes every required arm
once, accepts an adapter error as a missing arm, rejects unsafe, empty, or
receipt-mismatched raw evidence, and returns sealed staged evidence.
`RenderSkillEvalStage` and `DecodeSkillEvalStage` make the stop/resume boundary
durable without serializing producer-local roots. `Resume` accepts exact human
reviews plus a freshly recomputed caller snapshot, rechecks its seal, bindings,
and reconstructed raw trees, and produces the unpersisted schema-v1
safe summary without invoking the adapter. `RenderSkillEvalReport` produces
canonical JSON; it never writes a durable review, alters skills or fixtures, or
applies a proposal.

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
