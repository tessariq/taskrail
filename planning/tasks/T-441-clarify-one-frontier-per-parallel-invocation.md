---
id: T-441-clarify-one-frontier-per-parallel-invocation
title: Clarify one frontier per parallel loop invocation
status: todo
priority: low
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T11:39:14Z"
---

# T-441-clarify-one-frontier-per-parallel-invocation Clarify one frontier per parallel loop invocation

## Description

Spec gap. `loopParallelExecute` runs exactly one frontier per invocation, and
`--max-iterations` above `--parallel` is silently ignored, while the spec's
"remaining iteration budget" and "no new frontier" wording suggests successive
frontiers. The result reports no remaining task or `iteration_limit`. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- The spec and docs say whether a parallel invocation runs one frontier or several.
- If one: `--max-iterations` greater than `--parallel` in parallel mode is refused or reported as a dry-run warning, and the result names remaining eligible work.

## Verification Notes

- Dry-run and execution with `--parallel 2 --max-iterations 4` and four eligible tasks.

## Implementation Notes
