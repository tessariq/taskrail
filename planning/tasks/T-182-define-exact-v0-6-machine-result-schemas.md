---
id: T-182-define-exact-v0-6-machine-result-schemas
title: Define exact v0.6 machine result schemas
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-and-restore-commands
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-08-04T23:06:23Z"
---

# T-182-define-exact-v0-6-machine-result-schemas Define exact v0.6 machine result schemas

## Description

Define reusable strict types/encoders/decoders and code registries for v0.6
cancel, inventory, storage, migration warnings, and recovery before command
implementations consume them.

## Acceptance

- Schema-v1 envelopes enforce command/result/error exclusivity, nullable fields,
  string identities/generations, non-null arrays, warning objects, and
  unknown/missing rejection.
- Cancel, inventory, transition, eligibility/recovery, path blocker, unsupported
  path, snapshot, transaction, validation, and refusal-details objects expose
  exact named fields/enums.
- Stable warning/error registries include debt, scan, recovery_pending, partial,
  and rollback outcomes; APIs require identical dry-run/apply refusal
  classification.
- Error details can carry transition/eligibility/applied/scan facts and encoders
  guarantee one JSON document without free-form nested shapes.
- Inherited verify_pass_before_complete warning type remains byte/schema
  compatible while v0.6 outer results can add task_ref.

## Verification Notes

- Map every schema/object/code to strict construction, round-trip,
  unknown/null/missing/empty, and one-document golden fixtures.
- Mutation-test field/code registries before command integration.

## Implementation Notes
