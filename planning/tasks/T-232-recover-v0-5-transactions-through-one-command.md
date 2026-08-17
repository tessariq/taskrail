---
id: T-232-recover-v0-5-transactions-through-one-command
title: Recover v0.5 transactions through one command
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-277-add-durable-transaction-journals-and-recovery
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-17T12:18:28Z"
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
- A4. Typed snapshots recover mixed transactions containing managed logical,
  worktree-physical skill/runtime, and canonical absolute Git exclusion-store
  paths without conflating equal-looking strings or exposing a physical overlay
  as durable semantic data.

## Verification Notes

- A1: failure-injected fixtures for layout, local init/promotion, workflow pair,
  and ImportDraft v2 observe the expected mechanical action.
- A2: snapshot preview/apply parity and second-interruption tests provide durable
  recovery evidence.
- A3: race and alias substitutions prove exact `write_conflict` diagnostics and
  unchanged external bytes.
- A4: local init-with-skills failure fixtures bind semantic, assistant, runtime,
  and effective Git exclusion snapshots in one recoverable transaction.

## Implementation Notes

- 2026-08-17T12:18:12Z: Added the public recover boundary: taskrail recover <transaction-id> [--apply] [--json] previews and performs the single mechanically safe action the durable engine derives, acquires the mutation lock naming the transaction, maps engine refusals onto the registered machine codes with typed whole-set snapshot evidence, and reports logical managed, worktree-physical, and canonical absolute Git paths without conflating them. Accept without a shipped writer validator refuses validation_failed; the lock+fence crash combination (recover lock_held, lock clear fenced) is recorded as the created follow-up.
- 2026-08-17T12:18:28Z: verification pass
