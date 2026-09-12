---
id: T-174-run-the-v0-5-0-gap-and-drift-release-gate
title: Run the v0.5.0 gap and drift release gate
status: todo
priority: high
spec_ref: specs/v0.5.0.md#goals
dependencies:
    - T-248-run-cross-platform-workflow-contract-tests-in-ci
    - T-375-prevent-git-fixture-cleanup-races
    - T-377-define-collision-free-post-spec-finding-identities
    - T-378-close-implementation-review-disposition-vocabulary
    - T-379-require-durable-results-for-review-adapter-delivery
    - T-380-close-parallel-integration-publication-boundaries
    - T-381-restore-portable-verification-predecessor-evidence
    - T-382-make-maintainer-skill-evaluations-executable
    - T-383-accept-real-git-object-ids-in-skill-evaluations
    - T-384-prevent-decomposition-from-mutating-unmet-preconditions
    - T-385-make-skill-evaluation-scenarios-match-their-claims
    - T-389-require-layout-2-for-every-semantic-writer
    - T-390-describe-loop-as-executing-in-command-help
    - T-394-make-skill-eval-baselines-executable
    - T-395-separate-publication-from-dirty-worktree
    - T-396-read-baseline-validate-envelope
    - T-397-grade-on-managed-path-confinement
    - T-398-ignore-artifacts-on-committed-init
    - T-413-adopt-pre-staged-baseline-arms
    - T-414-publish-skill-eval-report
updated_at: "2026-09-09T14:07:00Z"
last_verification_id: "525eef8e0e6f39b6ce2558ed2536b5cd"
last_verification_result: fail
last_verified_at: "2026-09-08T19:22:51Z"
last_verification_previous_id: "f414774034bb61736378e18cf4520e7d"
---

# T-174-run-the-v0-5-0-gap-and-drift-release-gate Run the v0.5.0 gap and drift release gate

## Description

Perform the final v0.5.0 semantic gap, drift, exclusion, and release-readiness
review from a fresh implementation/spec/task snapshot after every implementation
and remediation task is complete. Do not tag or claim current until it passes.

## Acceptance

- Every goal, feature, caution, recommendation, and exclusion is classified
  in one release matrix against implementation, tests, packaged Agent Skills,
  lightweight SDD handoff, task-local loop policy, docs, and release notes.
- Coverage is 100 percent, every structural signal has a disposition, and
  independent semantic/adversarial review leaves no blocker.
- Representative decomposition, existing-task review, pre-start replan, and
  follow-up-routing evidence demonstrates the T-251 sizing behavior for aligned,
  oversized, fragmented, and integration-owner cases. Sampling supports the
  release judgment but does not claim exhaustive semantic proof.
- Every structural sizing-adjacent signal is dispositioned as evidence for review,
  not as proof of semantic size; counts, graph shape, coverage, and successful
  mechanical bundle validation cannot by themselves satisfy the semantic gate.
- Final task, spec, decomposition, and workflow review evidence carries valid
  role-mandated prompt-template bindings; built-in and replacement publication
  pass, stale replacement publication fails, and final planning reviews bind the
  current prompt/spec bytes without overstating reviewer attestation.
- Full formatting, vet, tests, race, cross-build, parity, bodies, freshness,
  validation, release build/snapshot, checklist, clean tree, CI, Planning,
  CodeQL, migration, Agent Skills conformance, SDD/loop-policy drift, unsupported
  legacy-input refusal, and native Linux/macOS/Windows packaged evidence passes.
- Opt-in local skill install/refresh/discovery, narrow exclusion, storage-neutral
  execution, product-only local delivery, and consented/unconsented promotion
  evidence passes without a `--without-skills` surface or implicit install path.
  Local delivery evidence includes reported transient paths, absence of incidental
  private planning provenance, unchanged Git identity/configuration, frozen generic
  policy, outcome-required product-byte cases, caller-authorized local identity/path
  commit-metadata cases, auxiliary-ref refusal, and delayed/current planning self-
  authorization refusal.
- Changed packaged skills have a committed safe candidate-versus-release summary
  with deterministic grades and human review. Outcome is pass or an explicitly
  disclosed valid strict schema-v1 waiver; fail/incomplete or malformed reports
  block, raw transcripts remain transient under the reported artifacts directory,
  and the committed report retains digest-only raw references. A waiver also
  requires explicit human disposition that its approver is the authorized v0.5.0
  release operator; the report cannot establish that authority itself. Exactly one
  committed report must match the final tested product, candidate package, and
  fixture inventory and bind the evaluated candidate/baseline binaries; final HEAD
  descends from its tested HEAD with only task/state/skill-eval-review bookkeeping
  changes, while zero or
  multiple current reports block selection among committed sessions. Discarded retries remain disclosed human
  evaluation risk rather than a fabricated append-only ledger.
- Every current-version blocker becomes a standalone remediation task and direct
  gate dependency, explicitly not a follow-up-of the gate; the gate stops and
  later restarts the review on fresh bytes. Cancelled dependencies never satisfy
  it.
- Changelog/README become final only after all other criteria; final verify occurs
  only with no open release remediation, and tagging remains a maintainer action.
- The source-checkout bootstrap cleanup is completed through T-258 and searches
  prove `scripts/autonomous-loop/` plus every live invocation/test reference is
  absent from the tagged tree and release artifacts.

## Verification Notes

- Map each criterion to the semantic matrix, command logs, remote URLs,
  Agent Skills/SDD/loop-policy/prompt-binding evidence, native/manual reports,
  Git/task dependency observations, and final fresh verification.
- Record representative sizing fixtures and explicit dispositions for each
  structural signal, including the semantic evidence used or the reason it does
  not establish task size.
- In a sandbox create a standalone blocker, add only the gate-to-remediation
  dependency, prove no cycle and gate ineligibility, complete it, then restart
  review.

## Implementation Notes

- 2026-08-28T16:14:33Z: Final fresh spec review found current v0.5 contract blockers tracked as direct dependencies T-377 through T-380; remediate them and restart the gate on fresh bytes.
- 2026-08-28T16:14:34Z: verification fail id bbffe73d3b28bd8eee671d2220af9351 previous none completion none
- 2026-08-29T11:09:23Z: Direct remediation dependencies T-377 through T-381 are completed and pass-verified; restart the v0.5.0 release gate on current bytes.
- 2026-08-29T11:10:12Z: Release gate blocked: the paired skill-evaluation registry lacks executable scenario and deterministic-oracle bindings, and the runner cannot stage completed arms before required human comparisons; T-382 owns remediation before a fresh restart.
- 2026-08-29T11:10:19Z: verification fail id f414774034bb61736378e18cf4520e7d previous bbffe73d3b28bd8eee671d2220af9351 completion none
- 2026-08-31T11:48:31Z: The T-382 evaluator capability remediation is complete; the final model-backed run exposed a new scenario-integrity blocker owned by T-385.
- 2026-08-31T11:48:41Z: Release gate blocked: the final skill-evaluation scenarios did not establish the committed and positive-path states their prompts claimed, allowing deterministic passes over semantically mismatched fixtures; T-385 owns remediation before one fresh final run.
- 2026-09-08T18:00:48Z: T-385 remediation is complete and pass-verified: evaluation scenarios now establish the committed clean HEAD and tracked-work subject their prompts claim, a focused model-backed replay graded candidate pass through the caller-owned provider adapter, and cross-platform execution is restored with runs failing closed on unexpected adapter errors and sandbox digests no longer racing Git's own internals. All eleven direct dependencies are completed and the full CI matrix is green. The gate restarts on current bytes with one fresh untouched complete paired release evaluation; provider execution and credentials remain operator-owned.
- 2026-09-08T19:22:39Z: Release gate blocked on two confirmed current-version defects plus the operator-owned final evaluation. T-389: v0.5.0 never raises the layout to 2 (currentLayoutVersion is still 1, fresh init writes a layout-1 marker) and only 3 of the semantic writers gate on layout 2, so a layout-1 repository accepts v0.5 lifecycle, loop-policy, and verification writes and a v0.4.0 binary then silently erases loop_policy and loop_reason. T-390: taskrail loop --help describes the executing command as a preview. Both are direct gate dependencies. The final paired skill-evaluation report is still absent and remains operator-owned: no rendered report exists, the four local sessions stopped at the human worksheet, and packaged skills changed since v0.4.0. Restart the gate on fresh bytes after remediation.
- 2026-09-08T19:22:51Z: verification fail id 525eef8e0e6f39b6ce2558ed2536b5cd previous f414774034bb61736378e18cf4520e7d completion none
- 2026-09-09T14:07:00Z: All 13 direct dependencies are completed and pass-verified; the gate is restartable on current bytes. The remaining item is the operator-owned paired skill-evaluation run.
