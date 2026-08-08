---
id: T-269-enforce-machine-result-schema-drift-checks
title: Enforce machine result schema drift checks
status: completed
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-268-decode-the-strict-common-machine-envelope
updated_at: "2026-08-08T18:20:30Z"
---

# T-269-enforce-machine-result-schema-drift-checks Enforce machine result schema drift checks

## Description

Make machine-contract drift a deterministic failure whenever a constructed command,
registered result, warning, error, or exit policy no longer agrees with the
normative v0.5 inventory and strict common decoder.

## Acceptance

- A1. Every constructed JSON-capable command is matched to exactly one normative
  inventory entry, while declared future commands remain distinguishable and do
  not masquerade as implemented coverage.
- A2. Checks fail for missing or duplicate command registration and for mismatched
  result type, warning subset, error subset, or report-result exit policy.
- A3. The drift check is deterministic and participates in the repository's normal
  verification path before incompatible machine output can ship.

## Verification Notes

- A1: compare the constructed command set with the inventory and inspect the
  explicit treatment of planned commands.
- A2: introduce one controlled perturbation for each mismatch class, observe a
  focused failure, then restore the registered contract.
- A3: run the normal verification entry point twice and observe stable diagnostics
  and ordering.

## Implementation Notes

- 2026-08-08T18:20:11Z: Added a deterministic v0.5 machine-contract drift check: MachineJSONState records whether an inventory entry publishes the common envelope, the inherited pre-v0.5 shape, or nothing; CheckMachineRegistrations holds the CLI's --json command set to that inventory; CheckMachinePublication holds one document to the strict decoder plus its entry's result shape, warning subset, error subset, and report-result exit policy. Runs in go test ./... .
- 2026-08-08T18:20:30Z: verification pass
