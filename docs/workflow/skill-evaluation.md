# Skill Evaluation

Maintainer contract for evaluating Taskrail's packaged Agent Skills. This is an
active v0.5 roadmap surface until T-218 ships.

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
