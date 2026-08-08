---
id: T-262-preview-and-validate-active-spec-transition-plans
title: Preview and validate active-spec transition plans
status: todo
priority: high
spec_ref: specs/v0.6.0.md#guided-active-spec-transition
dependencies:
    - T-179-resolve-stable-task-references-across-every
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-188-add-cancellation-provenance-and-dependency
    - T-252-validate-reviewed-decomposition-bundles
    - T-259-add-explicit-agent-mode-and-structured-help
updated_at: "2026-08-08T11:20:18Z"
---

# T-262-preview-and-validate-active-spec-transition-plans Preview and validate active-spec transition plans

## Description

Implement read-only transition inventory and strict digest-bound plan preview so
operators can disposition every affected open task before changing the active
spec.

## Acceptance

- Inventory reuses the exact `spec diff` anchor delta, lists every affected live
  open source-spec task, and emits the complete plan contract without choosing an
  action, allocating an ID, or writing.
- Strict plan decoding requires complete ordered repoint/cancel/retain actions,
  valid target anchors/reasons/rationales, exact spec/task/ledger digests, and a
  current reviewed ImportDraft v2/v3 bundle when task creation is requested.
- Preview constructs and validates the complete candidate, including allocation
  and candidate-wide cancellation dependencies, while preserving terminal and
  archived history and publishing nothing.
- Text/JSON results and stale/invalid error classifications match the exact v0.6
  schemas and remain stable under task/spec/review/plan races.

## Verification Notes

- Golden-test inventory and every action shape, missing/duplicate actions,
  best-effort rename non-inference, digest races, reviewed draft variants,
  dependency closure, stable allocation, and zero-write guarantees.

## Implementation Notes
