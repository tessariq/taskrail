---
id: T-217-release-interrupted-active-work-safely
title: Release interrupted active work safely
status: todo
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-158-bind-completion-and-verification-with-stable
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-05T20:24:26Z"
---

# T-217-release-interrupted-active-work-safely Release interrupted active work safely

## Description

Add a direct-operator recovery transition that relinquishes interrupted or
deliberate-rework active work without inventing blocker history or cancelling it.

## Acceptance

- `task release` accepts exactly one live `in_progress` task plus a bounded portable reason; dry-run and common JSON report the exact candidate and refusal class.
- Apply changes status to `todo`, clears only the matching current-task pointer, appends a timestamped Implementation Note, reprojects state, validates, and publishes transactionally.
- Completion/verification history, identity, body, dependencies, spec reference, and paired loop policy remain unchanged. No blocker or cancellation provenance is created.
- Other statuses, archived targets, ambiguous references, delegated loop ownership, and retained recovery transactions refuse without writes.
- Human/agent recovery docs distinguish release from unblock, cancel, reopen, and automatic continuation.

## Verification Notes

- Exercise every status/storage/pointer combination, reason grammar/YAML quoting, preview/apply parity, task/state rollback, lock delegation, and exact byte preservation.
- Run an interrupted-loop sandbox that releases work, proves ordinary next-task eligibility, and records no fabricated blocker history.

## Implementation Notes
