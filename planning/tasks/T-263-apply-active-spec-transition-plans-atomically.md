---
id: T-263-apply-active-spec-transition-plans-atomically
title: Apply active-spec transition plans atomically
status: todo
priority: high
spec_ref: specs/v0.6.0.md#guided-active-spec-transition
dependencies:
    - T-163-validate-and-apply-importdraft-v2-transactionally
    - T-180-make-semantic-publication-durably-transactional
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-187-create-opaque-tasks-through-commands-and
    - T-192-protect-archived-history-across-all-semantic
    - T-262-preview-and-validate-active-spec-transition-plans
updated_at: "2026-08-08T11:20:25Z"
---

# T-263-apply-active-spec-transition-plans-atomically Apply active-spec transition plans atomically

## Description

Apply one already-previewable active-spec transition as a durable transaction
covering task dispositions, reviewed task creation, and active-state projection.

## Acceptance

- Recheck every plan/spec/ledger/task/review digest and all destination,
  allocation, lock, recovery, and candidate invariants immediately before the
  first write; activating or changing any input invalidates the entire plan.
- Publish repoints, cancellations, reviewed live tasks, and target active state as
  one transaction. Retain actions write nothing; task-local loop fields and all
  unrelated or terminal/archived task bytes remain unchanged.
- Generate cancellation timestamps only during apply, preserve reviewed task
  content and implicit hold, and report stable references plus exact before/after
  actions and created IDs.
- Ordinary failure rolls back all writes; interruption, partial publication, and
  rollback races retain sufficient common recovery evidence without rewriting
  plan or review inputs.

## Verification Notes

- Failure-inject every publication boundary and race each bound input; prove
  complete apply, complete rollback, explicit recovery, candidate validity, and
  no partial activation/task disposition.

## Implementation Notes
