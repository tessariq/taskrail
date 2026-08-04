---
id: T-183-validate-cancellation-generation-and-archive
title: Validate cancellation generation and archive eligibility metadata
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-eligibility-and-verification-metadata
dependencies:
    - T-175-implement-arbitrary-width-generated-task-keys
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-158-bind-completion-and-verification-with-stable
updated_at: "2026-08-04T23:06:23Z"
---

# T-183-validate-cancellation-generation-and-archive Validate cancellation generation and archive eligibility metadata

## Description

Implement pure candidate validation, warning classification, and
archive-eligibility evaluation for cancellation provenance and v0.5/v0.6
completion/verification shapes before migration or writer commands consume
them.

## Acceptance

- Cancellation fields validate exact pairing, canonical timestamp, reason
  grammar, cleared completion/verification set, and legacy no-provenance debt
  without writing.
- Completion/verification validation covers every strict v0.6 pass/fail
  generation shape, completed-fail hybrid debt, and every valid generation-less
  v0.5 shape without confusing fresh open tasks.
- Warning classification emits exact debt codes/task refs/remediation and each
  modeled complete/cancel/verify transition predicts warning disappearance.
- Completed eligibility requires matching pass IDs/positive generations;
  cancelled eligibility requires valid provenance/no open dependent; adopted
  archive debt is valid warning but ineligible.
- Unlisted partial metadata is a violation and evaluation never trusts
  timestamps or old notes/artifacts as eligibility.

## Verification Notes

- Map criteria to exhaustive status/field/debt/transition tables, malformed
  timestamps/reasons, generations beyond 64 bits, open dependents, and adopted
  archive cases.
- Property-test that only specified transitions remove warnings and no
  failing/partial state becomes eligible.

## Implementation Notes
