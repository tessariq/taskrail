---
id: T-418-fail-an-in-run-skill-eval-raw-root-ancestor-swap
title: Fail an in-run skill-eval raw-root ancestor swap loudly
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-415-confine-executed-skill-eval-raw-roots-against
updated_at: "2026-09-15T21:20:13Z"
---

# T-418-fail-an-in-run-skill-eval-raw-root-ancestor-swap Fail an in-run skill-eval raw-root ancestor swap loudly

## Description

Deferred independently meaningful outcome: Deferred from T-415 review (finding S1, low, security): the Execute-time confinement check is a non-atomic pre-check. A component on the raw-root ancestor chain can be replaced with a symlink after the walk but before or during writing, because os.MkdirAll follows symlinks on existing components and the evaluated skill is untrusted code running concurrently inside adapter.Run; provider evidence written after the swap lands outside the artifact tree, and the immediate post-run digest reads through the symlink and matches, so the escape surfaces only at stage resume. Spec anchor: specs/v0.5.0.md maintainer-skill-release-evaluations pins raw evidence beneath the artifacts directory. Outcome: fail a mid-run ancestor swap loudly in-run, by re-running skillEvalConfinedRawRoot after adapter.Run before the outcome receipt is written and/or migrating the walk to an O_NOFOLLOW/openat-style descent, covering the same shape on the adoption and stage-resume paths, with a regression test that swaps an ancestor inside the adapter and asserts refusal.

This task owns integrated delivery of the deferred outcome and its invariant after T-415-confine-executed-skill-eval-raw-roots-against's verification.

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes
