---
id: T-288-inspect-local-planning-storage-read-only
title: Inspect local planning storage read-only
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-287-initialize-ignored-local-planning-storage-durably
updated_at: "2026-08-08T14:23:08Z"
---

# T-288-inspect-local-planning-storage-read-only Inspect local planning storage read-only

## Description

Implement read-only `local status` and `local path` over the active storage context.
These commands expose enough mode, origin, path, exclusion, drift, and promotion
readiness data for operators and agents without probing config or physical semantic
files directly.

## Acceptance

- `local status` reports exact storage mode/roots, strict origin and current Git
  snapshots, drift classification/warning, ordered exclusions, promotion readiness,
  and closed-shape violations in text and schema-1 JSON.
- `local path` reports mode, config/storage roots, logical specs/planning paths, and
  physical prompts/artifacts/runtime directories from the same context; its local
  `artifacts_dir` equals `status` storage output for one stable snapshot.
- Both commands recheck every consumed config/origin/Git/exclusion path before
  output, create no scaffold, exclusion, lock, or transaction file, and never
  bootstrap an uninitialized repository.
- Descendant cwd and linked-worktree output identify the effective scope without
  exposing lock/delegation secrets or converting logical semantic paths to overlay
  paths.
- Malformed origin, mixed state, incompatible layout, or changed snapshots return
  the applicable common read error without partial output.

## Verification Notes

- Golden-test text and exact JSON in committed/local, drift/no-drift, descendant,
  linked-worktree, malformed-origin, and uninitialized fixtures.
- Assert status/path artifact equality and deterministic exclusion/violation order.
- Compare filesystem, index, and Git metadata digests before/after each success,
  refusal, and injected snapshot race.

## Implementation Notes
