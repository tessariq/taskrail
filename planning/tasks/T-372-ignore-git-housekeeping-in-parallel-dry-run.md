---
id: T-372-ignore-git-housekeeping-in-parallel-dry-run
title: Ignore Git housekeeping in parallel dry-run mutation test
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-333-preview-deterministic-parallel-clone-batches
updated_at: "2026-08-26T15:12:43Z"
completion_id: "88e9f076520a76f417c3dd156eecfdeb"
last_verification_id: "07d3d3530f3557f60c704712541f279e"
last_verification_result: pass
last_verified_at: "2026-08-26T15:12:43Z"
last_verified_completion_id: "88e9f076520a76f417c3dd156eecfdeb"
---

# T-372-ignore-git-housekeeping-in-parallel-dry-run Ignore Git housekeeping in parallel dry-run mutation test

## Description

Keep the parallel loop dry-run no-mutation test portable when read-only Git
inspection triggers implementation-owned object packing or ref-cache maintenance.
The test must continue to detect worktree, index, ref, and Git-configuration
changes that violate dry-run behavior.

## Acceptance

- The parallel dry-run test ignores only semantically inert Git object-store and
  cache housekeeping while still comparing repository-visible files and the
  existing semantic Git snapshot.
- Native macOS CI passes without changing production loop behavior.

## Verification Notes

- Run the focused loop preflight test repeatedly, the full suite, and exact-head
  native macOS CI.
- Preserve byte checks for all other Git control files, compare refs/index/status,
  root refs, and configuration semantically, and run full object-integrity fsck.

## Implementation Notes

- 2026-08-26T15:12:37Z: Narrow the parallel dry-run no-mutation oracle to ignore only Git object packing and info/refs cache bytes while preserving semantic Git, control-file, worktree, and full fsck integrity checks.
- 2026-08-26T15:12:43Z: verification pass id 07d3d3530f3557f60c704712541f279e previous none completion 88e9f076520a76f417c3dd156eecfdeb
