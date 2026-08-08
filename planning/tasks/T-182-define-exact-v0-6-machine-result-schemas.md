---
id: T-182-define-exact-v0-6-machine-result-schemas
title: Define exact v0.6 machine result schemas
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-and-restore-commands
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-08-08T08:40:49Z"
---

# T-182-define-exact-v0-6-machine-result-schemas Define exact v0.6 machine result schemas

## Description

Define common envelope generation 2 and strict result/detail registries for v0.6
stable identity, cancel, dependency, inventory, storage, transition/readiness,
embedded skill/help inspection, migration warnings, and recovery before command
implementations consume them.

## Acceptance

- Every v0.6 command emits schema version 2 with inherited outer member names and
  strict command/result/error exclusivity; no command emits schema version 1.
- Cancel preview/apply, dependency add/remove, inventory, transition, eligibility/recovery, path blocker, unsupported
  path, snapshot, transaction, validation, and refusal-details objects expose
  exact named fields/enums.
- Stable warning/error registries include debt, scan, recovery_pending, partial,
  and rollback outcomes; APIs require identical dry-run/apply refusal
  classification.
- Error details can carry transition/eligibility/applied/scan facts and encoders
  guarantee one JSON document without free-form nested shapes.
- Envelope-v2 replacements add stable task/dependency refs and storage/index facts
  while preserving inherited typed snapshot path kinds, verify-order, and local
  warning meanings.
- Task-targeting prompt contract v2 adds exact `TASK_REF`/`TASK_STORAGE` context;
  schema-2 loop dry-run and iteration prompt objects add required
  `contract_version`, while schema-1 objects and explicit prompt v1 stay unchanged.
- The complete warning union names exact unchanged and task-ref-extended v0.5
  variants plus identity/storage variants and their command-specific subsets.
- Transition inventory/preview/apply, release-check, embedded skill list/show,
  and structured help expose their exact field order, nullability, collection
  ordering, result/error classification, and stable error codes. A not-ready
  release report is a registered completed non-zero result.
- Agent mode selects existing schema-version-2 output only; it creates no second
  envelope generation or command-specific result variant.

## Verification Notes

- Map every schema/object/code to strict construction, round-trip,
  unknown/null/missing/empty, and one-document golden fixtures.
- Mutation-test field/code registries before command integration.

## Implementation Notes
