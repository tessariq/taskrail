---
id: T-218-add-maintainer-skill-release-evaluations
title: Add maintainer skill release evaluations
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-166-publish-workflow-review-index-and-reports-with-cas
    - T-254-make-every-packaged-skill-storage-neutral
updated_at: "2026-08-08T08:40:49Z"
---

# T-218-add-maintainer-skill-release-evaluations Add maintainer skill release evaluations

## Description

Add maintainer-owned behavioral evaluation cases and a manual release procedure
for every packaged skill while keeping provider execution, credentials, and
automatic source modification outside Taskrail core and installed packages.

## Acceptance

- Eval definitions live outside the embedded skill tree and bind stable cases, inputs/digests, expected outcomes, deterministic assertions, and human-review questions for every shipped skill.
- Changed skills compare candidate bytes with the prior released skill; new skills
  compare with no skill. Runs retain raw outcomes under ignored artifacts and
  produce a safe digest-bound summary candidate with missing data, adapter/model
  identity, deterministic grades, and human review. T-174 owns the final committed
  release report after this evaluator has shipped.
- A provider-neutral maintainer procedure runs the complete registered suite manually before
  skill-heavy releases; credential absence/incomplete runs are explicit, never a
  passing check. T-249 owns the separately reviewed waiver outcome.
- Analysis may generate patch proposals and rerun candidates only in an isolated workspace. It cannot alter fixtures/assertions, shipped skills, mirrors, tracked state, commits, or select/apply a winner.
- Required no-model CI separately checks frontmatter, references, command/flag existence, lifecycle/JSON policy, nested resources, provider independence, and package parity.
- Every registered behavioral case runs once in its declared storage mode, and
  every shipped skill has registered committed and local cases;
  local cases detect direct logical-path opens, physical-overlay reconstruction,
  force-added metadata, and incorrect product-only delivery. They inspect commit
  subjects/bodies/trailers, author/committer identity, Git configuration changes,
  attribution/signing/hook policy, incidental private planning references, and
  caller-owned identity/path authorization versus generic frozen repository-policy
  conventions, outcome-required product-byte exceptions, and delayed or current-
  run self-authorization attempts.
- The committed report is strict schema version 1 with the exact ordered
  tested-HEAD/candidate+baseline-executable/product, candidate/baseline, fixture,
  intended/observed adapter/model, paired case-run, deterministic, human-review, and null-waiver
  fields. Each arm binds its raw digest without a path; producer-owned bytes remain
  under the status-reported transient artifacts directory.
- The exact checked registry represents every globally unique case ID, binds every
  shipped skill in committed and local storage, requires exact prompt/expected-observation/
  assertions/human-review-question metadata, and uses the normative
  domain-separated tree digests for candidate/baseline packages, skill subtrees,
  fixture inventory/cases, and the complete canonical per-session/per-case raw
  evidence subtree. Candidate bytes bind clean release-gate HEAD and baseline bytes
  bind exact peeled `refs/tags/v0.4.0` commit
  `62be3b13a67cbba51a4900b7ef6e192a645eb12d`. Exact outcome precedence rejects
  empty, single-storage, or unpaired false passes and every snapshot/executable/aggregate/case/digest
  mismatch. Base report behavior requires null waiver; T-249 owns non-null waiver
  decoding and `waived` outcome. Human safe-summary review owns free-form path
  portability without making historical validity depend on later storage context.

## Verification Notes

- Seed positive/negative/recovery/boundary cases for every packaged skill across
  committed and local fixtures, and prove eval assets are absent from
  `init --with-skills` output.
- Run one sandbox candidate-vs-release suite, capture blind/human comparison plus
  deterministic grades, propose but do not apply a patch, and validate the
  uncommitted report candidate; T-174 reruns and publishes final release evidence.
- Strict positive/mutation fixtures cover field order/types/nullability, sorting,
  portable case grammar/strict assertion-question types, registry closure, paired
  candidate/baseline arms, zero-run truthful identity,
  committed/local case closure, digest preimages, tested ancestor and per-arm
  executable binding, baseline-failure improvement, outcome precedence,
  favorable-session refusal, digest-only raw evidence, and empty false passes.

## Implementation Notes
