---
id: T-415-confine-executed-skill-eval-raw-roots-against
title: Confine executed skill-eval raw roots against symlinked ancestors
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-413-adopt-pre-staged-baseline-arms
updated_at: "2026-09-15T21:20:13Z"
completion_id: "e14936823b8f00c149cdb2cbd9a5ab44"
last_verification_id: "8922bfd131bac3ef8cf49d86633efb70"
last_verification_result: pass
last_verified_at: "2026-09-15T21:20:13Z"
last_verified_completion_id: "e14936823b8f00c149cdb2cbd9a5ab44"
---

# T-415-confine-executed-skill-eval-raw-roots-against Confine executed skill-eval raw roots against symlinked ancestors

## Description

Executed skill-evaluation arms create their raw roots with a plain `MkdirAll`
and hand the path to the adapter without checking that every component between
the artifact root and the raw root is a real directory. T-413 added that
ancestor-confinement guard (`skillEvalConfinedRawRoot`) to the adoption path
and to stage-resume validation, but the executed-arm path in `Execute` still
trusts the artifact tree: a symlinked or non-directory ancestor component
beneath the artifact root can direct an executed arm's raw evidence outside
the artifacts directory the spec pins it to, and the relocation surfaces only
at resume, after provider evidence has been written through the moved root.

One independently meaningful outcome: every raw root the skill-evaluation
runner uses — executed or adopted — is confinement-checked at `Execute` time
before any evidence is written, so raw evidence can never land outside the
session's artifact tree. Spec anchor: `specs/v0.5.0.md` pins raw evidence
trees under the artifacts directory and refuses outside-root files.

## Acceptance

- `Execute` refuses an executed arm whose raw-root path from the artifact root
  traverses a symlinked or non-directory component, with a loud error naming
  the arm, before the adapter is invoked or any evidence is written.
- Legitimate executed-arm roots — components that do not exist yet (created by
  `MkdirAll`) and real directory ancestors — still execute without false
  refusals.
- The existing adoption-path and stage-resume confinement checks are
  unchanged, and the refusal wording is consistent with them.
- No report schema, digest preimage, or outcome-precedence change.

## Verification Notes

- Cheapest sufficient evidence per criterion: a unit test that replaces a
  skill ancestor beneath the artifact root with a symlink to a byte-identical
  external copy and expects `Execute` to refuse the executed arm before the
  adapter is invoked (a counting adapter proves zero invocations), plus a
  green run proving a not-yet-existing ancestor tree still executes.
- Record later evidence paths after verification.

## Implementation Notes

- 2026-09-15T21:20:07Z: Executed skill-eval arms are confinement-checked at Execute time: runSkillEvalArm refuses a raw root whose path from the artifact root traverses a symlinked or non-directory ancestor before the adapter is invoked, with wording consistent with the adoption and stage-resume refusals. Evidence: focused and full Go tests, gofmt, vet, and taskrail validate pass; one bounded review wave (General, Go, Security lanes) with candidate validation and fresh disposition verification.
- 2026-09-15T21:20:13Z: verification pass id 8922bfd131bac3ef8cf49d86633efb70 previous none completion e14936823b8f00c149cdb2cbd9a5ab44
