---
id: T-269-enforce-machine-result-schema-drift-checks
title: Enforce machine result schema drift checks
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-268-decode-the-strict-common-machine-envelope
updated_at: "2026-08-08T14:23:08Z"
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
