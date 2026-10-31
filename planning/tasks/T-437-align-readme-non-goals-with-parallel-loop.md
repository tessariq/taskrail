---
id: T-437-align-readme-non-goals-with-parallel-loop
title: Align README non-goals with the parallel loop
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T11:39:13Z"
---

# T-437-align-readme-non-goals-with-parallel-loop Align README non-goals with the parallel loop

## Description

Doc gap. README.md lists "Not a sandbox, container, or worktree
orchestrator" and "Not a background daemon, distributed worker pool, or
multi-lane scheduler", while `loop --parallel` creates private clones, runs up
to n workers concurrently and serially integrates them. The spec's closing
paragraph now names the bounded isolated-clone batch and marks parallel
batches and review delivery as experimental in v0.5; the README still
contradicts it. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- README non-goals name the bounded, foreground `loop --parallel` batch as the explicit exception.
- README and docs/loop.md label `--parallel` above one and `--delivery review` as experimental in v0.5, matching the spec.

## Verification Notes

- Read README.md, docs/loop.md, and the spec closing paragraph side by side.

## Implementation Notes
