---
id: T-324-harden-native-filesystem-portability-coverage
title: Harden native filesystem portability coverage
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies: []
updated_at: "2026-08-15T09:10:43Z"
---

# T-324-harden-native-filesystem-portability-coverage Harden native filesystem portability coverage

## Description

Strengthen the native filesystem confidence behind repository discovery,
locking, durable publication, and recovery. Keep native Linux, Windows, and
macOS runners authoritative; make platform capabilities and skipped behavior
visible; and remove duplicated portable-mode interpretation across durable
layers.

## Acceptance

- Focused regression tests cover the recent macOS root alias, Windows path,
  mode, directory-sync, and portable spec-path failures where the native runner
  can exercise them.
- CI runs a named verbose filesystem portability suite on every native matrix
  runner so capability skips are visible in job logs.
- Durable transaction recovery is exercised after forcibly terminating a real
  helper process at a persisted publication phase.
- Durable filesystem snapshots and transaction journals use one shared
  platform-aware portable-mode canonicalization rule.
- Targeted tests, `go vet ./...`, `go test ./...`, and `taskrail validate` pass.

## Verification Notes

- `task test:filesystem`, `go vet ./...`, `task test`, `task build:cross`,
  `task check:skills`, `task check:task-bodies`, and `taskrail validate` pass.
- Manual portability verification passed on 2026-08-15, including a fresh
  process-termination recovery run and target test-binary cross-compilation.
- Native Windows and macOS runtime results are verified by the post-push CI
  matrix; local cross-compilation proves only target compilation.

## Implementation Notes

- Added a verbose native portability suite for durable filesystem, transaction,
  lock, path, separator, and CRLF regressions, with capability skips visible in
  each matrix leg.
- Added platform-specific Windows and macOS regressions, real subprocess-death
  recovery coverage, and shared platform-aware mode canonicalization.
- Durable transaction tests now probe directory-sync support, so unsupported
  filesystems refuse visibly without turning the native matrix red.
- 2026-08-15T09:10:31Z: verification pass
