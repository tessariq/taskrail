---
id: T-168-parse-and-validate-an-optional-autonomous-run
title: Parse and validate an optional autonomous run policy
status: todo
priority: high
spec_ref: specs/v0.5.0.md#optional-autonomous-run-policy
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-04T21:32:13Z"
---

# T-168-parse-and-validate-an-optional-autonomous-run Parse and validate an optional autonomous run policy

## Description

Implement `AUTONOMY.tsv` parsing, static validation, and existing-writer
integration as an optional human-owned allowlist. Keep lifecycle truth in tasks
and preserve policy intent and exact bytes.

## Acceptance

- Parsing requires UTF-8 without BOM and at most 1 MiB, then enforces exact tab
  grammar, comments, line diagnostics, canonical path boundaries, allowed values,
  unique IDs, and non-empty control-free rationales while normalizing line
  endings only for parsing.
- Static validation enforces active run rows, narrow off-spec dependency-context
  holds, earlier unresolved dependencies, terminal history, parked skip
  provenance, and cancelled dependency behavior.
- Validate handles absence without creation; layout 1 refuses policy use, and init
  never creates the file.
- Rename rewrites IDs transactionally; repoint and activation evaluate the exact
  resulting invariant and reject invalid state without changing policy intent.
- Writers preserve all existing policy bytes unless a later loop task invokes
  separately specified authorized insertion.

## Verification Notes

- Map criteria to size/BOM/UTF-8, LF/CRLF/comment snapshots, line diagnostics,
  symlink/reparse cases, policy/status/dependency matrices, and writer fault
  tests.
- Compare absent init, layout-1 refusal, rename/repoint/activate, and deliberate
  pruning preview without unintended writes.

## Implementation Notes
