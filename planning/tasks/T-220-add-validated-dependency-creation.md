---
id: T-220-add-validated-dependency-creation
title: Add validated dependency creation
status: todo
priority: high
spec_ref: specs/v0.6.0.md#cancellation-transition-and-provenance
dependencies:
    - T-179-resolve-stable-task-references-across-every
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-185-upgrade-repositories-transactionally-to-layout-3
updated_at: "2026-08-05T20:24:45Z"
---

# T-220-add-validated-dependency-creation Add validated dependency creation

## Description

Complement narrow dependency removal with safe addition so operators can redesign
tracked work without hand-editing increasingly strict task frontmatter.

## Acceptance

- `task dependency add` resolves target/dependency through the unified ledger, supports dry-run/common JSON, and appends exactly one stable reference to a live open target.
- Duplicate, self, cycle, missing, ambiguous, cancelled-dependency, archived-target, alias, and recovery-pending cases refuse before writes with stable codes.
- Candidate validation uses complete-ledger semantics, preserves existing dependency order and task-local loop fields, reprojects STATE, and publishes through the durable transaction protocol.
- Add/remove share exact result/error types and dry-run/apply classification. Replacement and bulk editing remain excluded.

## Verification Notes

- Cover generated/opaque/legacy refs, live/archive dependencies, all refusals, cycle matrices, ordering, YAML quoting, preview purity, rollback, and concurrent ledger changes.
- Run add then remove end to end and prove eligibility/state projection returns to the original semantic outcome.

## Implementation Notes
