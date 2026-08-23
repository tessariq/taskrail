---
id: T-355-ignore-transient-git-locks-in-cli-snapshots
title: Ignore transient Git locks in CLI snapshots
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-244-publish-streamed-loop-results-out-of-band
updated_at: "2026-08-23T19:53:29Z"
completion_id: "11e31b85bed7a14de395b01780d6c285"
last_verification_id: "7fc7ffe620041520bf520d5e3f7608f3"
last_verification_result: pass
last_verified_at: "2026-08-23T19:53:29Z"
last_verified_completion_id: "11e31b85bed7a14de395b01780d6c285"
---

# T-355-ignore-transient-git-locks-in-cli-snapshots Ignore transient Git locks in CLI snapshots

## Description

Make CLI smoke-test repository snapshots insensitive to transient Git lock files
that background maintenance can create or remove while the snapshot walk runs.

## Acceptance

- Snapshot comparisons ignore `.git` lock files without ignoring ordinary
  repository files.
- A lock disappearing between directory enumeration and inspection does not fail
  the snapshot.
- The macOS loop result-file smoke test and full CI matrix pass.

## Verification Notes

- Add focused coverage for a transient `.git/objects/maintenance.lock`.
- Run the CLI package, full Go checks, and exact-head macOS CI.

## Implementation Notes

- 2026-08-23T19:53:24Z: Ignored transient .git lock entries and disappearance races; focused and full local checks pass.
- 2026-08-23T19:53:29Z: verification pass id 7fc7ffe620041520bf520d5e3f7608f3 previous none completion 11e31b85bed7a14de395b01780d6c285
