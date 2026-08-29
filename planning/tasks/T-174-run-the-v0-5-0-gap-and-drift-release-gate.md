---
id: T-174-run-the-v0-5-0-gap-and-drift-release-gate
title: Run the v0.5.0 gap and drift release gate
status: blocked
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
updated_at: "2026-08-29T11:10:19Z"
last_verification_id: "f414774034bb61736378e18cf4520e7d"
last_verification_result: fail
last_verified_at: "2026-08-29T11:10:19Z"
last_verification_previous_id: "bbffe73d3b28bd8eee671d2220af9351"
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
