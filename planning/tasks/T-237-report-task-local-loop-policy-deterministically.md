---
id: T-237-report-task-local-loop-policy-deterministically
title: Report task-local loop policy deterministically
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-local-loop-policy
dependencies:
    - T-168-parse-and-validate-an-optional-autonomous-run
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-06T13:46:30Z"
---

# T-237-report-task-local-loop-policy-deterministically Report task-local loop policy deterministically

## Description

Implement read-only loop-policy inventory and held-dependency diagnostics over the
validated policy model without inventing rows for undecodable files.

## Acceptance

- A1. `task loop list` reports every decodable task in canonical full-ID order with
  exact effective policy, source, reason, disposition, eligibility, and held closure.
- A2. Wholly undecodable files appear only as ordered path-bearing violations;
  decodable invalid tasks use an invalid row and the completed report exits nonzero.
- A3. The command is storage-neutral, read-only, and does not change ordinary
  lifecycle eligibility or selection.

## Verification Notes

- A1: policy/dependency graph fixtures compare exact text and schema-1 rows.
- A2: malformed identity/status/policy cases prove row-versus-violation behavior
  and report-result exit classification.
- A3: repository/Git snapshots before and after list prove zero writes.

## Implementation Notes
