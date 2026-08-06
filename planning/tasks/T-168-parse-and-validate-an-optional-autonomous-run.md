---
id: T-168-parse-and-validate-an-optional-autonomous-run
title: Manage task-local loop policy
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-local-loop-policy
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-04T21:32:13Z"
---

# T-168-parse-and-validate-an-optional-autonomous-run Manage task-local loop policy

## Description

Add direct operator commands to inspect and manage unattended authorization on
individual tasks: `task loop list`, `task loop allow`, `task loop hold`, and
`task loop clear`. Keep loop intent beside task lifecycle data without allowing
delegated agents, body authors, or imports to grant themselves authority.

## Acceptance

- The task frontmatter fields `loop_policy` and `loop_reason` are paired: both are
  present or both absent, policy is exactly `allow` or `hold`, and a reason is
  trimmed UTF-8 of 1 through 512 bytes with no newline, control character, or
  concrete ignored-artifact path. Missing fields mean implicit hold with the
  deterministic default reason.
- `task loop list` is deterministic and read-only in text and JSON, reports every
  task in canonical full-ID order, and exposes status, active-spec membership,
  explicit/default source, effective policy, reason, eligibility, ordered held
  dependencies, disposition, and deterministic violations.
- `task loop allow`, `hold`, and `clear` use the repository transaction protocol,
  mutate only the paired fields of one `todo` or `blocked` task, preserve all
  other task bytes except the task timestamp, re-project state, and validate the
  all-or-none candidate. Other lifecycle statuses refuse without mutation.
- Loop-policy mutation is direct-operator-only. A process joined through delegated
  lock ownership refuses these commands regardless of its other capabilities or
  task-field write set.
- Every mutator supports dry-run and common JSON, reports old/effective/candidate
  policy through the exact result shape. Initialized repositories classify
  preview/apply semantic refusals identically; uninitialized apply may perform
  separately disclosed local bootstrap while dry-run remains write-free.
- Validation rejects malformed or half-present fields. Existing lifecycle, body,
  rename, repoint, import, and review writers preserve the pair exactly and cannot
  use their write sets to change it; `STATE.md` never duplicates the pair.
- A legacy `AUTONOMY.tsv` is refused as unsupported input with explicit removal
  and task-command remediation; Taskrail never parses, imports, or rewrites it.

## Verification Notes

- Map criteria to paired/half-present/malformed fields, all lifecycle statuses,
  implicit versus explicit holds, exact task-byte preservation, state projection,
  list text/JSON output, lock contention, and transaction fault tests.
- Exercise delegated refusal and legacy TSV refusal, then compare snapshots across
  lifecycle, body, rename, repoint, import, and review writers.

## Implementation Notes
