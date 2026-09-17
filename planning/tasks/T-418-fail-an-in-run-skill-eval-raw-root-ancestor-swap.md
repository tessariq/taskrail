---
id: T-418-fail-an-in-run-skill-eval-raw-root-ancestor-swap
title: Fail an in-run skill-eval raw-root ancestor swap loudly
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-415-confine-executed-skill-eval-raw-roots-against
updated_at: "2026-09-17T20:25:00Z"
last_verification_id: "1eac8e1ce823607df2cefc270381e773"
last_verification_result: fail
last_verified_at: "2026-09-15T21:36:16Z"
---

# T-418-fail-an-in-run-skill-eval-raw-root-ancestor-swap Fail an in-run skill-eval raw-root ancestor swap loudly

## Description

Make skill-evaluation execution fail closed when an adapter-run namespace swap
invalidates raw-root confinement. A candidate or required baseline arm whose
raw-root ancestor is replaced during `adapter.Run` must not yield a usable run,
sealed stage, or later report; the refusal occurs before the runner records the
adapter outcome receipt or accepts a digest of the relocated tree. Adoption of
pre-staged baselines and stage resume retain their confinement checks before
facts or digests are consumed.

This closes the detection and evidence-acceptance gap deferred from T-415. The
v0.5 contract requires accepted raw evidence to remain beneath the reported
artifacts root, but this task must not claim that a post-run check prevents or
rolls back bytes an untrusted adapter already wrote after a namespace swap.
Only a stronger handle-bound write boundary can establish that stronger
prevention claim, and the implementation and evidence must distinguish
prevention from detection. Provider/model execution, release evaluation, and
T-174 dependency changes are out of scope.

## Acceptance

- For each executed arm type, a deterministic adapter that replaces a raw-root
  ancestor with a symlink or platform-equivalent non-directory at a controlled
  point during `adapter.Run` causes execution to return a loud confinement
  error naming the affected arm/root context. No usable `SkillEvalStage`, run
  record, runner-written outcome receipt, or digest/grade from the relocated
  namespace is accepted after the swap.
- A pre-staged baseline whose raw-root ancestor is substituted before adoption
  is refused before its facts, outcome receipt, or raw digest are consumed, and
  the adopted arm cannot silently become a successful stage. The existing
  executed candidate/baseline behavior remains valid when no substitution
  occurs.
- After a valid stage is sealed, substituting an ancestor with a byte-identical
  external raw tree before resume still causes resume to refuse before it
  returns a report. The refusal is based on confinement, not on a digest
  mismatch or a timing-dependent race.
- Legitimate raw roots with real directory ancestors or not-yet-existing
  ancestors continue to execute, seal, resume, and produce the existing report
  and digest shapes without a schema or outcome-precedence change.
- The regression evidence states whether the chosen implementation prevented
  an attempted post-swap write or detected the swap after such a write; a
  post-run detection check is never presented as retroactive write prevention.

## Verification Notes

- Build a credential-free runner fixture whose adapter performs the ancestor
  replacement synchronously inside `Run` and then attempts a raw write. Assert
  the executed path returns the arm-specific confinement refusal, produces no
  usable stage or runner receipt, and does not accept a relocated digest.
  Inspect any external sentinel only to distinguish prevention from detection;
  do not use sleeps, polling, or timing races.
- Reuse valid pre-staged baseline evidence for an adoption case, substitute its
  ancestor before the adopted arm is read, and assert refusal before facts,
  outcome, or digest consumption. Execute a valid stage, replace an ancestor
  with a byte-identical external copy, and assert resume refuses before report
  construction. These are boundary tests over the public runner/stage APIs.
- Keep positive controls for missing raw ancestors, real directories, normal
  executed candidate and baseline arms, and an unchanged stage/resume report;
  compare the durable raw digest and decoded report shape rather than only
  checking that calls return.
- Run focused credential-free Go tests, `gofmt`, vet, and Taskrail validation.
  Do not run provider/model-backed evaluation or T-174; record the focused test
  paths and the prevention-versus-detection observation in the implementation
  verification record.

## Implementation Notes

- 2026-09-15T21:36:13Z: Pre-start sizing gate: unauthored placeholder follow-up - outcome, acceptance, and verification sections retain unedited TODO scaffold lines, so a verified result cannot be reached without unresolved scope and improvised criteria cannot support a pass. Needs human-approved task author body authoring or reviewed decomposition before execution.
- 2026-09-15T21:36:16Z: verification fail id 1eac8e1ce823607df2cefc270381e773 previous none completion none
- 2026-09-17T20:25:00Z: Authoring approved: replace the placeholder body with a bounded outcome, acceptance, and verification plan; implementation and verification remain outstanding.
