---
id: T-382-make-maintainer-skill-evaluations-executable
title: Make maintainer skill evaluations executable
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-08-29T11:09:41Z"
---

# T-382-make-maintainer-skill-evaluations-executable Make maintainer skill evaluations executable

## Description

Make the provider-neutral maintainer evaluation flow executable as trustworthy
release evidence. Every registered case must define enough deterministic sandbox
setup and grading behavior to run its candidate and required baseline arms, while
the runner must preserve those frozen results for a later human comparison rather
than requiring favorable human conclusions before any arm executes.

The independently meaningful outcome is one complete paired evaluation that can
run from a clean release snapshot, pause at the human boundary, and then render a
canonical digest-bound report without rerunning or inventing evidence.

## Acceptance

- Every registered skill-evaluation case has a strict, validated executable
  scenario and deterministic oracle mapping for its authored assertions; missing,
  unknown, ambiguous, or non-mechanical grading inputs fail before provider use.
- A maintainer adapter can execute all candidate and required baseline arms once,
  retain safe digest-bound arm results and a human-review worksheet, and stop
  before comparison without fabricating `same|better|worse` values.
- After explicit human comparisons are supplied, report construction consumes the
  exact frozen staged results without rerunning an arm and rejects stale snapshot,
  executable, skill, fixture, case, raw-evidence, or review bindings.
- The reviewed contract and maintainer documentation define candidate/baseline
  executable selection and truthful comparison semantics for skills with no
  v0.4.0 baseline; placeholder comparisons cannot produce a passing report.
- A sandboxed complete-registry test covers every candidate and required baseline
  arm, deterministic pass/fail/incomplete grading, interruption and missing-arm
  handling, staged-evidence tampering, human-review resumption, and canonical safe
  report rendering without exposing raw transcripts or producer-local paths.

## Verification Notes

- Begin with a reviewed v0.5 contract clarification if the two-phase staged result
  or case format changes normative report or registry semantics.
- Use registry/schema mutation tests for executable scenarios and oracles, runner
  tests for execute-then-review resumption and stale binding refusal, and one
  complete sandboxed producer run with a fake deterministic adapter before the
  final model-backed release evaluation.
- Record later deterministic logs, raw-tree digests, review worksheet, safe report
  candidate, and manual-test evidence paths without committing provider output.

## Implementation Notes
