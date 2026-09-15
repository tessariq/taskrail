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
v0.4.0 baseline classification, and incomplete local fixtures. Baseline
classification covers the storage mode as well as the skill, and a
baseline-required scenario may use only the v0.4.0 command surface: local storage
and later flags do not exist there, so such an arm would fail for lacking a
command and read as candidate improvement.

Each case has a strict executable `scenario`: the fixture source, a sandbox name
equal to its case ID, setup commands, and action command vectors. Commands are
only documented `taskrail` or `git` invocations. `fixture/seed.json` is a
validated concrete initialization recipe: it requires `git init`, selects the
documented committed or local `taskrail init --json` form, and local seeds carry
their decoy and provenance bytes. Setup must also establish the state the prompt
claims: a `git commit` action, so the agent starts from a real `HEAD` with a
clean worktree instead of an unborn repository full of untracked seed files, and
a `taskrail task new` action, so a positive, negative, recovery, or boundary
request has a concrete tracked-work subject to exercise rather than degrading
into a passing refusal. Neither counts if the command only prints help or runs
a dry run. A case that does not is refused before any provider runs.
Its `oracle` maps every authored assertion
exactly once to a supported mechanical predicate over action facts. The supported
predicates are `command-exit-zero`, `taskrail-validation-pass`,
`git-worktree-clean` (the named Git command exits zero and the observed worktree
state is empty before and after it; an unchanged *dirty* worktree fails), and
`git-managed-paths-only` (every changed path, tracked or not, lies under the
managed planning directory, `HEAD` does not move, no ref is created or moved, and
the path listing was actually observed — an unobserved listing fails rather than
reading as an unchanged tree), and `git-publication-only` (tracked bytes are
unchanged before and after, so a skill that only publishes cannot rewrite what it
reviewed). Git does not report ignored paths, so a local-storage write under
`.taskrail/local/` is outside what the managed-path rule can mechanically confirm;
the human review questions cover it. The registered cases use the latter, because these skills are asked to
transition tracked work and publish review bundles: under `git-worktree-clean` a
skill fails for producing its own required output, and whether a run reaches that
output at all varies, so the grade describes the run rather than the skill. Every
declared setup action must additionally be observed with exit code zero, or the
arm grades `fail`. An adapter returns structured facts containing exact
command argv/exit code, stdout/stderr, filesystem and Git before/after digests,
the changed path list, validation result, and storage paths, then writes the same canonical `facts.json`
receipt beneath its raw root. The runner rejects missing, extra, fabricated, or
receipt-mismatched facts and derives the deterministic grade only from predicate
evaluation; adapters do not supply assertion names or grades. Semantic claims
such as ambiguity handling, authority, or safe repair belong only in
`human_review_questions`. Spec-review cases must additionally direct their runs
at disposition authority and unchanged-byte repeat visibility and ask the human
to compare the actual agent transcript with the published bundle's declared
round history and disposition claims, because mechanical predicates certify only
supplied-bundle consistency, never transcript completeness or decision
identity. Case and registry fixture digests use
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
   Invoke each bound executable directly; do not resolve an unverified same-name
   executable through `PATH`.
   Execute every declared setup and action command, retain the stub or provider
   transcript as raw evidence, and record only actual command facts.
3. Render and retain the canonical sealed stage beside the producer-local raw
   evidence, then stop for a human worksheet. The stage contains no producer-local paths;
   resume reconstructs them from current input and refuses a changed receipt or
   raw tree. Do not render a final report or invent a comparison here.
 4. Decode and resume from that exact staged evidence after human review, or run the
   publication driver over it: `PublishSkillEvalReport` reads the sealed
   `stage.json` plus an answers file (the overall `human_review` and each case's
   `comparison` and `human_review`), reconstructs the staging run's input from
   explicitly caller-supplied current bindings — the tested head, the product
   snapshot, the two bound executables, and the current candidate and baseline
   skill digests — plus the current registry and the current raw trees, and
   writes the committed report to
   `<planning-dir>/reviews/skill-evals/v0.5.0/<session-id>/report.json`. The
   driver re-reads the current executable bytes and recomputes the fixtures
   digest itself; no binding is echoed back from the stage under validation.
   No adapter arm runs; answers that omit a staged case, name an unregistered
   one, or supply a comparison the case's staged completeness does not permit
   fail before anything is written, and republishing the same stage and
   answers writes byte-identical output. A completed paired
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
receipt-mismatched raw evidence, and returns sealed staged evidence. Every
executed arm also records its adapter-declared outcome in a canonical
runner-written `outcome.json` receipt inside its raw tree, covered by the
arm's raw digest, because that declaration exists nowhere else once the
adapter call returns. A
baseline-required case may instead be enumerated in the input's adopted list:
its baseline arm is adopted from the raw tree a prior session already staged
beneath the current session's baseline raw root, re-deriving the grade from the
staged facts receipt through the case's own predicates and recomputing the raw
digest without invoking the adapter. Reuse preserves the original outcome:
adoption reads the staged tree's outcome receipt and keeps the outcome the
original execution recorded, so an originally incomplete or failed baseline arm
stays that way no matter what the re-derived grade says. Adoption is opt-in and
verified exactly
like execution — an unregistered, non-baseline, duplicate, missing, empty,
noncanonical, or unconfined staged tree (one whose path from the artifact
root traverses a symlinked or non-directory ancestor component), or a staged
tree whose outcome receipt is missing, noncanonical, or unsupported fails the
run, and stage resume re-checks that confinement — candidate arms are never
adoptable, and the report schema is unchanged; disclose adopted arms in the
report's human review summary.
`RenderSkillEvalStage` and `DecodeSkillEvalStage` make the stop/resume boundary
durable without serializing producer-local roots. `Resume` accepts exact human
reviews plus a freshly recomputed caller snapshot, rechecks its seal, bindings,
and reconstructed raw trees, and produces the unpersisted schema-v1
safe summary without invoking the adapter. `RenderSkillEvalReport` produces
canonical JSON; it never writes a durable review, alters skills or fixtures, or
applies a proposal. `PublishSkillEvalReport` is the maintainer-facing driver
that closes the loop: it answers the worksheet across any number of sittings,
feeds the sealed stage and the answers through `Resume` together with the
caller's freshly recomputed current bindings, and writes the rendered report
as the one committed release artifact, refusing any current snapshot,
executable, skill, fixture, registry, or raw-tree drift before a write, so
the human comparison boundary is a real stopping point rather than a
single-call convenience.

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
