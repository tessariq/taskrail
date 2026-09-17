---
id: T-419-refuse-skill-eval-adoption-and-resume-on-artifact
title: Refuse skill-eval adoption and resume on artifact-root ancestor swaps
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-418-fail-an-in-run-skill-eval-raw-root-ancestor-swap
updated_at: "2026-09-17T21:46:09Z"
---

# T-419-refuse-skill-eval-adoption-and-resume-on-artifact Refuse skill-eval adoption and resume on artifact-root ancestor swaps

## Description

Deferred independently meaningful outcome: Anchor-and-above swap detection currently covers executed arms in-run (T-418) while adoption (adoptSkillEvalBaselineArm) and stage resume keep only the component walk, which resolves through a symlinked or renamed artifact root and cannot see it. Close the cross-session gap with actionable criteria: (1) seal the artifact-root anchor identity (type and dev+inode facets) into the stage at Execute time, extending the frozen v0.5 stage schema (schema_version/report/worksheet/seal per specs/v0.5.0.md) through a spec change with backward-compatible handling of already-sealed stages; (2) adoption and resume refuse, before facts, outcome receipt, or digest consumption, when the current anchor or any ancestor above it changed type or identity versus the sealed pin, including byte-identical external copies and inode-preserving renames; (3) execute-time paths use the pinned absolute anchor for every downstream resolution so an adapter chdir during Run cannot rebase a relative ArtifactRoot (pre-existing rebasing observation: relative roots let a chdir redirect runner receipts outside the artifact root with err=nil); (4) regressions: adoption-path and resume-path anchor-swap refusals, chdir+relative-root refusal, positive controls for legitimate restored trees and caller-stable symlinked anchors. Out of scope: provider/model execution, T-174.

This task owns integrated delivery of the deferred outcome and its invariant after T-418-fail-an-in-run-skill-eval-raw-root-ancestor-swap's verification.

## Acceptance

- `SkillEvalRunner.Execute` seals the artifact-root anchor identity into the
  stage at Execute time: the anchor's type facet plus its device and inode
  identity facet, and the same two facets for every ancestor above it up to the
  filesystem root, are persisted in the sealed stage alongside the frozen v0.5
  fields, through a spec change to the frozen stage schema
  (`schema_version`/`report`/`worksheet`/`seal` per specs/v0.5.0.md). Stages
  already sealed by the pre-change schema without the new anchor facts decode
  and resume through documented backward-compatible handling instead of being
  rejected or silently re-pinned.
- Adoption (`adoptSkillEvalBaselineArm`) and stage resume refuse, before any
  facts, outcome receipt, or digest is consumed, when the current anchor or any
  ancestor above it changed type or identity versus the sealed pin. The refusal
  fires for a byte-identical external copy of the tree and for an
  inode-preserving rename, and is a confinement refusal rather than a digest
  mismatch or timing-dependent race. No adopted arm, resumed stage, or report
  is produced from the swapped namespace.
- Executed arms resolve every downstream path from the pinned absolute anchor
  captured before the run, so an adapter `chdir` during `Run` cannot rebase a
  relative `ArtifactRoot`: the executed path refuses loudly rather than letting
  runner receipts land outside the artifact root with `err=nil`, and unchanged
  absolute roots keep their existing behavior.
- Regression coverage ships for the adoption-path and resume-path anchor-swap
  refusals and the chdir-plus-relative-root refusal, with positive controls for
  a legitimately restored tree (same type and identity facts re-established
  where the platform allows) and for a caller-stable symlinked anchor that does
  not change across the run, both continuing to adopt, execute, seal, and
  resume with the existing report and digest shapes.

## Verification Notes

- Criterion 1 (sealed anchor facts): in a credential-free Go test, run
  `Execute` against a real temporary artifact root, render the stage, and
  assert the persisted anchor facts match direct `os.Lstat` and `os.SameFile`
  observations of the anchor and each ancestor. Decode a legacy already-sealed
  stage fixture that lacks the new fields via `DecodeSkillEvalStage` and assert
  the documented backward-compatible resume path. Update specs/v0.5.0.md for
  the schema extension and prove structure with
  `go run ./cmd/taskrail validate --json`.
- Criterion 2 (adoption/resume refusal): extend the deterministic fixtures in
  internal/taskrail/skill_eval_adoption_test.go and the stage resume tests:
  substitute the anchor or an ancestor with a byte-identical external copy
  before the adopted arm is read, and rename the anchor preserving its inode
  before `Resume` decodes the sealed stage. Assert both refuse before fact,
  outcome, or digest consumption and yield no adopted arm, report, or digest
  from the relocated namespace, using the public runner/stage APIs only.
- Criterion 3 (executed-path pinned root): in a focused runner test, give the
  input a relative `ArtifactRoot` and an adapter whose `Run` performs an
  `os.Chdir` before returning. Assert the executed path returns the loud
  confinement refusal and writes no receipt outside the artifact root
  (`err=nil` redirection is impossible), and keep a positive control where no
  chdir occurs or the root is absolute and the run seals unchanged.
- Criterion 4 (regressions and controls): run the focused credential-free
  suite `go test ./internal/taskrail -run 'SkillEval'` plus `gofmt -l .`,
  `go vet ./...`, and `task validate`; assert the new adoption, resume, and
  chdir cases are present and fail against a revert of the refusal logic.
  Include the restored-tree and caller-stable symlinked-anchor positive
  controls, compare the durable raw digest and decoded report shape rather
  than only exit codes, and record the test paths and evidence in the
  implementation verification record. Provider/model execution and T-174 stay
  out of scope.

## Implementation Notes
