---
id: T-432-make-loop-result-file-diagnostics-match-schema
title: Make loop result-file diagnostics match the schema
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T11:39:11Z"
---

# T-432-make-loop-result-file-diagnostics-match-schema Make loop result-file diagnostics match the schema

## Description

Bug. The loop result file deviates from the exact diagnostic schema:
`storage` is encoded as `{"Mode": ..., "Root": ...}` instead of `mode`/`root`
because `LoopStorageSnapshot` (internal/taskrail/loop_preflight.go) has no JSON
tags; `validation` reports `valid:false` with an empty `violations` list when
the repository is valid and only mutation violations exist (`loopValidation`
in internal/taskrail/loop_execute.go); and the delivery violation message
"T-312 integrity violations prevent delivery" leaks an internal task key.
Dry-run uses a separate, correctly tagged type, so tests over dry-run do not
catch this. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- The result file's `storage` object has exactly `mode` and `root`.
- `validation.valid` is false only with at least one violation that explains it.
- No user-facing loop message contains an internal task key.
- A contract test decodes a real result file strictly against the spec field lists.

## Verification Notes

- Unit: marshal `LoopStorageSnapshot`; contract test on a result file produced by `LoopExecute`.

## Implementation Notes
