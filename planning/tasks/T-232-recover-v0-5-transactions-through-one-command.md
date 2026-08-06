---
id: T-232-recover-v0-5-transactions-through-one-command
title: Recover v0.5 transactions through one command
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-06T13:46:30Z"
---

# T-232-recover-v0-5-transactions-through-one-command Recover v0.5 transactions through one command

## Description

Expose one explicit recovery boundary for every retained v0.5 durable transaction
without asking the operator or binary to choose semantic content.

## Acceptance

- A1. `recover <transaction-id>` derives exactly one of restore-original,
  accept-candidate, or clear-fence from recorded and current snapshots.
- A2. Preview is read-only; apply changes only candidate-valued components,
  validates the resulting coherent state, and is itself interruption-safe.
- A3. A live owner, changed/mixed byte, substituted ancestor, or invalid candidate
  refuses without overwrite and preserves complete evidence.

## Verification Notes

- A1: failure-injected fixtures for layout, local init/promotion, workflow pair,
  and ImportDraft v2 observe the expected mechanical action.
- A2: snapshot preview/apply parity and second-interruption tests provide durable
  recovery evidence.
- A3: race and alias substitutions prove exact `write_conflict` diagnostics and
  unchanged external bytes.

## Implementation Notes
