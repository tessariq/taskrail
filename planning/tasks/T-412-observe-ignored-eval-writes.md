---
id: T-412-observe-ignored-eval-writes
title: Record local-storage and ignored writes in skill-eval facts
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-09-11T16:28:51Z"
---

# T-412-observe-ignored-eval-writes Record local-storage and ignored writes in skill-eval facts

## Description

Skill-eval facts record `storage_paths` as empty for every action, including
local cases that wrote tasks, verify artifacts, and review proposals under
local storage, so `git-managed-paths-only` passes trivially there. Writes to
ignored locations (a self-ignoring `planning/.gitignore`, `.git/info/exclude`
edits) are invisible to grading, and agent scratch under `planning/` is
indistinguishable from CLI output. In session v050-final-20260910t103851z these
were caught only by transcript review.

Outcome: facts record local-storage and ignored-path writes, so human review is
not the only check.

## Acceptance

- Facts capture the CLI-reported storage root before and after the agent, and
  changes to ignored paths.
- A predicate or report field surfaces edits to `.git/info/exclude` or new
  ignore files.

## Verification Notes

- Runner tests with a synthetic arm exercising each write kind.

## Implementation Notes
