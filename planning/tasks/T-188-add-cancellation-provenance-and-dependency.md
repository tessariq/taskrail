---
id: T-188-add-cancellation-provenance-and-dependency
title: Add cancellation provenance and dependency remediation
status: todo
priority: high
spec_ref: specs/v0.6.0.md#cancellation-transition-and-provenance
dependencies:
    - T-185-upgrade-repositories-transactionally-to-layout-3
    - T-179-resolve-stable-task-references-across-every
    - T-183-validate-cancellation-generation-and-archive
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-182-define-exact-v0-6-machine-result-schemas
updated_at: "2026-08-04T23:06:23Z"
---

# T-188-add-cancellation-provenance-and-dependency Add cancellation provenance and dependency remediation

## Description

Add first-class cancellation and narrow dependency removal so abandoned work
has provenance without stranding open dependents.

## Acceptance

- Both writers acquire transaction ownership before target/dependent resolution
  and hold through revalidation/rollback.
- Cancel accepts live todo/in-progress/blocked or legacy adoption, round-trips
  reason/time, clears exact completion/verification fields, appends note, and
  reconciles state/blockers without disturbing another active task.
- Open dependents refuse with sorted diagnostics; new cancelled edges fail while
  migration debt remains warning/remediable.
- Dependency remove dry-run writes nothing; apply removes exactly one resolved
  edge from live open work, preserves remaining order, reprojects STATE,
  revalidates, and refuses additions/replacements/aliases/archive/absent edges.
- Exact schemas, YAML metacharacters, recovery, archive refusal, and no
  uncancel/reopen match spec.

## Verification Notes

- Map criteria to pre-read races, statuses/adoption/reasons/state/dependents,
  dry-run/order/projection, aliases/archive, and transaction faults.
- Manually prove dependent refusal/remediation and exact provenance.

## Implementation Notes
