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

The spec now states that one parallel invocation runs exactly one frontier:
`--max-iterations` greater than `--parallel` is `invalid_arguments`, and
remaining eligible work is reported, never launched. Today
`loopParallelExecute` runs one frontier while silently ignoring a larger
`--max-iterations`, and the result reports no remaining work. Found in the
2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- Parallel dry-run and execution with `--max-iterations` greater than `--parallel` return `invalid_arguments` before any workspace is created.
- A batch that leaves eligible allowed work reports it as remaining work in the parallel diagnostic.
- docs/loop.md describes the one-frontier rule.

## Verification Notes

- Smoke tests for the refusal; integration test with three eligible tasks and width two asserting the remaining task.

## Implementation Notes
