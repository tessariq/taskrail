---
id: T-266-select-temporary-loop-backend-per-invocation
title: Select the temporary loop backend per invocation
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-257-add-the-temporary-source-checkout-autonomous-loop
updated_at: "2026-08-08T12:15:10Z"
---

# T-266-select-temporary-loop-backend-per-invocation Select the temporary loop backend per invocation

## Description

Move temporary-loop backend selection from the reviewed task queue to one explicit
runner invocation option. Keep the queue focused on task order and run/hold policy,
with Claude as the documented default and OpenCode as an explicit alternative.

Follow-up derived from T-257-add-the-temporary-source-checkout-autonomous-loop's verification or discovery.

## Acceptance

- `scripts/autonomous-loop/run.sh` accepts `--backend claude|opencode`, defaults to
  Claude, uses the selected backend for every iteration, and rejects missing or
  unsupported values before preflight or agent launch.
- `queue.tsv` contains only `task_id`, `mode`, and `reason`; queue validation keeps
  all existing completeness, ordering, hold, and immutability guarantees without
  storing agent selection.
- Dry-run reports the invocation backend without launching it, and local guidance
  documents the default and explicit alternatives.
- Disposable shell tests cover the default and explicit backends, invalid and
  missing values, absent selected CLI, child failure, and read-only dry-run.

## Verification Notes

- Run `bash -n scripts/autonomous-loop/run.sh scripts/autonomous-loop/test.sh` and
  `scripts/autonomous-loop/test.sh`.
- Run `scripts/autonomous-loop/run.sh --check-queue`, repository validation,
  `go vet ./...`, and `go test ./...`.

## Implementation Notes

- This is operator-owned maintenance of the temporary loop itself and is not an
  unattended child task.
- Added invocation-wide `--backend claude|opencode` selection with a Claude
  default, explicit argument failures, backend-specific launch diagnostics, and
  dry-run reporting. Removed backend identity from the queue schema and preserved
  its task-order and run/hold policy.
- The disposable harness covers default Claude, explicit Claude and OpenCode,
  invalid, missing, conflicting, and unavailable backends, child failure, and the
  existing lifecycle and delivery postconditions. The shell harness, queue check,
  Go tests, vet, formatting, planning validation, task-body hygiene, skill parity,
  and diff checks pass.
- 2026-08-08T12:15:04Z: Added invocation-wide backend selection, simplified queue policy, and covered both supported CLIs and failure paths.
- 2026-08-08T12:15:10Z: verification pass
