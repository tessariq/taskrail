---
id: T-376-fix-native-workflow-contract-portability
title: Fix native workflow contract portability
status: completed
priority: high
spec_ref: specs/v0.5.0.md#skill-and-prompt-behavioral-contract-tests
dependencies:
    - T-248-run-cross-platform-workflow-contract-tests-in-ci
updated_at: "2026-08-28T15:34:00Z"
completion_id: "5f28971a4e5421404f03f283a3292e7c"
last_verification_id: "1ea36ea468cd72753ae25142ba905b43"
last_verification_result: pass
last_verified_at: "2026-08-28T15:34:00Z"
last_verified_completion_id: "5f28971a4e5421404f03f283a3292e7c"
---

# T-376-fix-native-workflow-contract-portability Fix native workflow contract portability

## Description

Follow-up derived from T-248-run-cross-platform-workflow-contract-tests-in-ci's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

Make the registry-selected workflow contract gate report intentional native
capability skips without confusing them with missing coverage, and keep Unix
containment assertions deterministic on both Linux and macOS.

## Acceptance

- A1. Selected tests that explicitly skip publish their test names and non-empty
  reasons, while missing terminal results and unreasoned skips still fail closed.
- A2. The forced Unix process-group termination case deterministically exercises
  graceful-then-forced cleanup on Linux and macOS without shell-specific signal
  behavior.
- A3. The repository-local workflow contract command passes on Linux, the runner
  cross-compiles for Windows, and native exact-HEAD CI remains a required gate
  before the downstream release task starts.

## Verification Notes

- A1: focused runner tests cover pass, explicit skip, absent terminal result, and
  missing skip-reason handling.
- A2: focused Unix containment tests exercise the Go helper and require forced
  termination after the configured grace period.
- A3: run `task test:workflow-contract`, cross-compile the runner tests for Windows,
  and retain exact-HEAD GitHub Actions run URLs before starting T-174.

## Implementation Notes

- 2026-08-28T15:33:55Z: Recorded explicit per-test capability skips without weakening fail-closed suite accounting, and replaced shell-dependent forced-termination setup with a readiness-bound Go helper and bounded cleanup assertion.
- 2026-08-28T15:34:00Z: verification pass id 1ea36ea468cd72753ae25142ba905b43 previous none completion 5f28971a4e5421404f03f283a3292e7c
