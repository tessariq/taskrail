---
id: T-359-make-containment-cleanup-timing-race-aware
title: Make containment cleanup timing race-aware
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-358-prevent-read-only-git-probes-from-refreshing-index
updated_at: "2026-08-24T07:55:55Z"
---

# T-359-make-containment-cleanup-timing-race-aware Make containment cleanup timing race-aware

## Description

Keep Unix containment timing coverage meaningful under Go race instrumentation
without relaxing the production descendant-termination contract.

Follow-up derived from T-358-prevent-read-only-git-probes-from-refreshing-index's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

The current fixture starts its one-second wall-clock bound before launching a
race-instrumented helper and deterministically reports about 1.03 seconds even
though the descendant is gone and containment evidence is correct.

## Acceptance

- The fixture measures the intended containment interval rather than unrelated
  race-instrumented helper startup, or uses a capability-aware bound that still
  detects the production grace-period regression.
- Descendant absence and termination evidence remain mandatory on every run; no
  production timeout, grace period, or process behavior changes.
- Focused repeated race runs and the full race package pass on Unix.

## Verification Notes

- Reproduce with `go test -race ./internal/taskrail -run
  '^TestLoopLaunchChildTerminatesDescendantsAfterLeaderExit$' -count=3`.
- Run focused normal/race repetitions, the full package race suite, full tests,
  vet, build, and cross-platform CI.

## Implementation Notes
