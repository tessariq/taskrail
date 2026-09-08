---
id: T-385-make-skill-evaluation-scenarios-match-their-claims
title: Make skill evaluation scenarios match their claims
status: blocked
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-08T11:56:17Z"
last_verification_id: "e2300a54329669025922479ac302c35b"
last_verification_result: fail
last_verified_at: "2026-09-08T11:56:17Z"
---

# T-385-make-skill-evaluation-scenarios-match-their-claims Make skill evaluation scenarios match their claims

## Description

Make each maintainer skill-evaluation scenario establish the repository state and
workflow inputs its prompt claims before provider execution. A mechanically
passing run must not treat an unborn repository with untracked seed files as a
"clean committed repository" or convert an absent positive-path subject into a
successful refusal case.

The independently meaningful outcome is a registry whose setup, prompt,
expected observation, oracle, and human worksheet all describe the same
executable scenario, so T-174 can run one trustworthy final evaluation.

## Acceptance

- Every committed case establishes a real clean `HEAD` before agent execution,
  while local cases preserve their configured storage, decoy, and provenance
  boundaries; setup failure or claimed-state mismatch refuses the arm before the
  provider is invoked.
- Positive, negative, recovery, and boundary requests contain the concrete
  subjects and preconditions needed to exercise those paths. A missing task,
  review, gap, drift, or other claimed subject cannot silently turn the case into
  a passing refusal scenario.
- Deterministic assertions distinguish an actually clean worktree from one that
  merely retains the same dirty digest and bind the observed actions needed for
  the authored expected observation; semantic authority questions remain
  explicitly human-owned.
- Registry and runner tests cover unborn/untracked committed fixtures, missing
  positive-path subjects, pre-provider refusal, and complete valid committed and
  local scenarios without weakening raw-receipt or sealed-stage validation.
- A focused model-backed replay demonstrates the corrected scenario boundary;
  T-174 owns the next untouched complete paired release evaluation.

## Verification Notes

- Begin with a reviewed v0.5 contract clarification if clean-worktree predicate
  or pre-provider setup semantics change.
- Use registry mutations and a recording fake adapter to prove contradictory
  scenarios fail before provider use, then run one representative committed and
  local replay in isolated sandboxes.
- Preserve the invalid sealed session only as ignored diagnostic evidence at
  `planning/artifacts/skill-evals/v0.5.0/v050-final-20260831t094858z/`; never
  publish it as the release report.

## Implementation Notes

- 2026-08-31: The fresh 32-case/54-arm release run sealed with every candidate
  deterministic grade passing, but human transcript review showed committed
  prompts running in unborn repositories with untracked seed files and several
  claimed positive paths absent. The unchanged-worktree oracle therefore proved
  stability of an already-dirty fixture rather than the claimed clean state.
- 2026-09-08T11:56:10Z: Deterministic scope is complete and green: the registry refuses a case whose setup does not commit a real HEAD and create a tracked-work subject, a failed setup action now grades the arm fail, git-worktree-clean requires an actually empty worktree, and all 32 cases execute end to end against the working binary. The remaining acceptance criterion is a focused model-backed replay through the caller-owned provider adapter; provider execution and credentials are operator-owned, so this unattended run did not invoke one.
- 2026-09-08T11:56:17Z: verification fail id e2300a54329669025922479ac302c35b previous none completion none
