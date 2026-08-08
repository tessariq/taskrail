---
id: T-270-migrate-inherited-read-only-machine-results
title: Migrate inherited read-only machine results
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-08T14:23:08Z"
---

# T-270-migrate-inherited-read-only-machine-results Migrate inherited read-only machine results

## Description

Move inherited read-only JSON commands to the common v0.5 envelope while
preserving each report's established meaning, deterministic payload, and explicit
non-zero result exceptions.

## Acceptance

- A1. Every inherited read-only `--json` command emits its exact registered result
  inside the common envelope with clean stdout and canonical command identity.
- A2. `validate`, coverage gates, loop-policy listing, and loop dry-run retain result
  envelopes when their completed report contract makes findings non-zero.
- A3. Inability to produce a report emits the command's registered error envelope,
  and equivalent text/JSON invocations retain exit-classification parity.
- A4. Read-only machine invocations remain side-effect-free, including commands
  whose completed reports exit non-zero.

## Verification Notes

- A1: compare representative inherited result payloads with their strict registered
  envelope goldens.
- A2: exercise every documented non-zero report exception and observe a result,
  not an error.
- A3: force argument, repository, and subject failures and compare text/JSON exits
  plus structured error output.
- A4: snapshot the repository before and after successful and gating invocations
  and observe no managed changes.

## Implementation Notes
