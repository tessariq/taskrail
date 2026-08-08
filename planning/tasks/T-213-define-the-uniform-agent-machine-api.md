---
id: T-213-define-the-uniform-agent-machine-api
title: Establish the uniform machine envelope and errors
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-269-enforce-machine-result-schema-drift-checks
updated_at: "2026-08-08T08:40:49Z"
---

# T-213-define-the-uniform-agent-machine-api Establish the uniform machine envelope and errors

## Description

Establish one schema-version-1 production boundary for JSON-capable commands so a
selected command always returns the normative success envelope or a structured
common error, with clean stdout and the same exit classification as human mode.

## Acceptance

- A1. A selected JSON-capable command emits exactly one schema-version-1 document
  with canonical command path, non-null warnings, and exactly one of its registered
  result or error; diagnostics do not contaminate stdout.
- A2. Argument, preflight, validation, conflict, partial-write, rollback, and
  recovery failures after command selection use registered common errors and exact
  details rather than prose-only failure output.
- A3. Human and JSON modes classify equivalent outcomes identically; writer
  refusals and inability to produce a promised report are errors, while explicitly
  gating completed reports remain results.
- A4. The envelope version governs the complete document and rejects an
  unregistered command/result/error combination before emission.

## Verification Notes

- A1: invoke representative commands through success and refusal paths and compare
  stdout with strict envelope goldens.
- A2: exercise each post-selection failure class and inspect its registered code,
  common details, snapshots, and recovery reference.
- A3: compare human/JSON exits, including one gating report and one writer refusal.
- A4: perturb a command/result/error registration and observe refusal before an
  incompatible document is emitted.

## Implementation Notes
