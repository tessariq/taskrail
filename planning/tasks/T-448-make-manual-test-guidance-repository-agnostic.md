---
id: T-448-make-manual-test-guidance-repository-agnostic
title: Make manual-test guidance repository-agnostic
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#storage-neutral-packaged-skills
dependencies: []
updated_at: "2026-11-02T23:26:02Z"
completion_id: "593bb0fd09f8f6b0bb9a761fae98ab5f"
last_verification_id: "5aa25e501099a485e38666ce616ab2bf"
last_verification_result: pass
last_verified_at: "2026-11-02T23:26:02Z"
last_verified_completion_id: "593bb0fd09f8f6b0bb9a761fae98ab5f"
---

# T-448-make-manual-test-guidance-repository-agnostic Make manual-test guidance repository-agnostic

## Description

Adopter skill audit P6 identifies a Taskrail-specific recipe in
autonomous-manual-test: "Exercise normal `${TASKRAIL:-taskrail} ...` flows."
The product under test may instead be a web application or another CLI, and
the adopter may document sandbox or container test modes.

Make the recipe exercise the changed product's acceptance criteria using the
repository's documented test modes, with a safe temporary sandbox fallback.
As part of that same recipe edit, address P10's repetition without weakening
the distinct prohibitions on committing artifacts and temporary test code.
Do not add container management or prescribe a new adopter workflow.

## Acceptance

- The recipe directs agents to the adopting repository's documented manual-test
  modes and to the affected product's user-facing flows, not Taskrail commands
  unless those commands are actually the subject of the task.
- Without documented modes, prefer a temporary sandbox; preserve acceptance-
  derived plans, non-pass outcomes for placeholder acceptance, cleanup, and
  artifact-root discovery through Taskrail's JSON result.
- State the no-commit rule for both ephemeral evidence and temporary test code
  clearly without redundant prohibitions. Keep actionable cleanup instructions.
- Preserve external-resource safety boundaries and package/copy parity.

## Verification Notes

- Check an adopter application task with documented test modes: its plan must
  exercise application acceptance criteria in those modes, not run Taskrail as
  the application under test. Check an adopter without a policy retains the
  sandbox fallback. Use disposable fixtures, not live infrastructure.
- Extend focused skill-contract coverage for generic product flows, policy
  lookup, cleanup, and both no-commit categories. Regenerate skill copies and
  run the focused tests plus `task check:skills`.

## Implementation Notes

- 2026-11-02T23:26:01Z: Manual-test guidance now selects adopter-documented modes and acceptance-derived product flows, with a temporary sandbox fallback. Preserved placeholder non-pass, JSON artifact discovery, cleanup, evidence and temporary-code no-commit rules, and external safety boundaries. Focused contracts, package parity, vet, full Linux tests, disposable interpretation scenarios, simplification, and independent review passed; review clarified the owned local sandbox exception.
- 2026-11-02T23:26:02Z: verification pass id 5aa25e501099a485e38666ce616ab2bf previous none completion 593bb0fd09f8f6b0bb9a761fae98ab5f
