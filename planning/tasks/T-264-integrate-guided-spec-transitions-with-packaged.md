---
id: T-264-integrate-guided-spec-transitions-with-packaged
title: Integrate guided spec transitions with packaged workflows
status: todo
priority: high
spec_ref: specs/v0.6.0.md#guided-active-spec-transition
dependencies:
    - T-196-integrate-stable-references-with-rename-prompts
    - T-260-inspect-the-embedded-skill-package
    - T-261-report-spec-release-readiness-read-only
    - T-263-apply-active-spec-transition-plans-atomically
updated_at: "2026-08-08T11:20:30Z"
---

# T-264-integrate-guided-spec-transitions-with-packaged Integrate guided spec transitions with packaged workflows

## Description

Integrate transition planning, release readiness, embedded skill inspection, and
agent-mode help into the packaged spec workflow and v0.6 documentation without
inventing another planning lifecycle.

## Acceptance

- `taskrail-spec` guides inventory, semantic disposition, reviewed decomposition,
  preview, apply, and release-check in order; it never treats rename candidates as
  decisions or bare activation as migration.
- Skills use structured help and embedded inspection for current command/package
  facts while retaining provider neutrality, storage neutrality, and exact mirror
  parity.
- Human/agent workflows, README, command help, release guidance, migration notes,
  and CHANGELOG consistently distinguish transition apply, state-only activation,
  read-only readiness, and maintainer-owned release.
- Integrated committed/local scenarios cover successful and refused transitions,
  retained off-spec work, reviewed new tasks, readiness before/after completion,
  and agent-mode output without hidden writes.

## Verification Notes

- Run behavioral skill evaluation, package parity, documentation drift checks,
  machine schema tests, and end-to-end committed/local transition sandboxes.

## Implementation Notes
