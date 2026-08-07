---
id: T-218-add-maintainer-skill-release-evaluations
title: Add maintainer skill release evaluations
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-166-publish-workflow-review-index-and-reports-with-cas
    - T-254-make-every-packaged-skill-storage-neutral
updated_at: "2026-08-05T20:24:33Z"
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
  commit a safe digest-bound summary with missing data, adapter/model identity,
  deterministic grades, and human review.
- A provider-neutral maintainer procedure runs relevant suites manually before
  skill-heavy releases; credential absence/incomplete runs are explicit, never a
  passing check. T-249 owns the separately reviewed waiver outcome.
- Analysis may generate patch proposals and rerun candidates only in an isolated workspace. It cannot alter fixtures/assertions, shipped skills, mirrors, tracked state, commits, or select/apply a winner.
- Required no-model CI separately checks frontmatter, references, command/flag existence, lifecycle/JSON policy, nested resources, provider independence, and package parity.
- Every applicable behavioral case runs against committed and local storage;
  local cases detect direct logical-path opens, physical-overlay reconstruction,
  force-added metadata, and incorrect product-only delivery.

## Verification Notes

- Seed positive/negative/recovery/boundary cases for every packaged skill across
  committed and local fixtures, and prove eval assets are absent from
  `init --with-skills` output.
- Run one candidate-vs-release suite, capture blind/human comparison plus deterministic grades, propose but do not apply a patch, and link the final untouched-case release report.

## Implementation Notes
