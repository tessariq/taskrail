---
id: T-248-run-cross-platform-workflow-contract-tests-in-ci
title: Run cross-platform workflow contract tests in CI
status: completed
priority: high
spec_ref: specs/v0.5.0.md#skill-and-prompt-behavioral-contract-tests
dependencies:
    - T-173-check-cross-surface-workflow-contract-integrity
updated_at: "2026-08-28T14:24:17Z"
completion_id: "2a68ac9633721ebe710756bea8a55e05"
last_verification_id: "769577d58b7ba6b4a29d277ce7908d31"
last_verification_result: pass
last_verified_at: "2026-08-28T14:24:17Z"
last_verified_completion_id: "2a68ac9633721ebe710756bea8a55e05"
---

# T-248-run-cross-platform-workflow-contract-tests-in-ci Run cross-platform workflow contract tests in CI

## Description

Run the portable deterministic workflow contract suite continuously on Linux,
macOS, and native Windows after the cross-surface registry is complete.

## Acceptance

- A1. CI executes the registry-selected lock/path/process/CLI/skill/prompt suites on
  all three operating systems, including local skill destination, exclusion,
  no-follow, collision, and discovery-path suites, without replacing required
  manual agent evidence.
- A2. Platform-specific unsupported guarantees are explicit skips with reasons;
  supported behavior cannot silently disappear from one matrix leg.
- A3. Required checks remain reproducible locally and do not add provider
  credentials, model calls, or stochastic grading to CI.

## Verification Notes

- A1: CI workflow runs and packaged smoke logs provide three-OS evidence.
- A2: registry-to-matrix checks fail when a supported case lacks an OS assignment.
- A3: local documented commands and credential scans prove deterministic execution.

## Implementation Notes

- 2026-08-28T14:24:01Z: Wired the registry-selected workflow contract runner into every native CI matrix leg and the planning/docs lane, added one reproducible Task target, explicit platform skip records, and fail-closed structural wiring guards.
- 2026-08-28T14:24:17Z: verification pass id 769577d58b7ba6b4a29d277ce7908d31 previous none completion 2a68ac9633721ebe710756bea8a55e05
