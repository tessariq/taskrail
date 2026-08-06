---
id: T-168-parse-and-validate-an-optional-autonomous-run
title: Define the task-local loop policy model
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-local-loop-policy
dependencies:
    - T-229-canonicalize-v0-5-lifecycle-and-task-identities
    - T-230-enforce-the-normative-v0-5-machine-schema
updated_at: "2026-08-04T21:32:13Z"
---

# T-168-parse-and-validate-an-optional-autonomous-run Define the task-local loop policy model

## Description

Define, parse, validate, and preserve the paired task-local authorization model.
Reporting and mutation commands are separate tasks T-237 and T-238.

## Acceptance

- The task frontmatter fields `loop_policy` and `loop_reason` are paired: both are
  present or both absent, policy is exactly `allow` or `hold`, and a reason is
  trimmed UTF-8 of 1 through 512 bytes with no newline, control character, or
  concrete ignored-artifact path. Missing fields mean implicit hold with the
  deterministic default reason.
- Loop-policy mutation is direct-operator-only. A process joined through delegated
  lock ownership refuses these commands regardless of its other capabilities or
  task-field write set.
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
