---
id: T-169-select-autonomous-work-through-policy-barriers
title: Select autonomous work through policy barriers
status: todo
priority: high
spec_ref: specs/v0.5.0.md#optional-autonomous-run-policy
dependencies:
    - T-168-parse-and-validate-an-optional-autonomous-run
updated_at: "2026-08-04T21:32:13Z"
---

# T-169-select-autonomous-work-through-policy-barriers Select autonomous work through policy barriers

## Description

Implement the exact runtime policy table, ordered barriers, and byte-preserving
bounded follow-up-row insertion used by unattended execution.

## Acceptance

- Absence falls back to active-spec selection; present policy is an allowlist
  processed in order with exact run, hold, skip, terminal, blocked, in-progress,
  and invalid-row actions and exit semantics.
- Valid hold and no-work are clean zero-exit handoffs; recovery/invalid conditions
  are non-zero and launch no child.
- Off-spec holds never launch and only block as valid dependency context; skipped
  in-progress work and cancelled dependencies cannot be bypassed.
- Existing presence, row bytes, comments, rationale, and order remain exact; at
  most two real selected-task follow-ups insert at the exact run/hold positions
  without line-ending rewrites.
- Repeated selection and authorized insertion remain deterministic under mixed
  status and dependency changes.

## Verification Notes

- Map every runtime-table row to setup/action/action-result/exit-code evidence and
  add mixed-policy ordering plus no-launch process observations.
- Use byte fixtures for LF/CRLF comments and each insertion position, including
  invalid additions and the two-row cap.

## Implementation Notes
