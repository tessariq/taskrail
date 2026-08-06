---
id: T-248-run-cross-platform-workflow-contract-tests-in-ci
title: Run cross-platform workflow contract tests in CI
status: todo
priority: high
spec_ref: specs/v0.5.0.md#skill-and-prompt-behavioral-contract-tests
dependencies:
    - T-173-check-cross-surface-workflow-contract-integrity
updated_at: "2026-08-06T13:46:30Z"
---

# T-248-run-cross-platform-workflow-contract-tests-in-ci Run cross-platform workflow contract tests in CI

## Description

Run the portable deterministic workflow contract suite continuously on Linux,
macOS, and native Windows after the cross-surface registry is complete.

## Acceptance

- A1. CI executes the registry-selected lock/path/process/CLI/skill/prompt suites on
  all three operating systems without replacing required manual agent evidence.
- A2. Platform-specific unsupported guarantees are explicit skips with reasons;
  supported behavior cannot silently disappear from one matrix leg.
- A3. Required checks remain reproducible locally and do not add provider
  credentials, model calls, or stochastic grading to CI.

## Verification Notes

- A1: CI workflow runs and packaged smoke logs provide three-OS evidence.
- A2: registry-to-matrix checks fail when a supported case lacks an OS assignment.
- A3: local documented commands and credential scans prove deterministic execution.

## Implementation Notes
