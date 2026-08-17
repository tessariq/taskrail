---
id: T-328-skip-recovery-apply-tests-when-directory
title: Skip recovery apply tests when directory durability is unsupported
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-232-recover-v0-5-transactions-through-one-command
updated_at: "2026-08-17T14:58:27Z"
---

# T-328-skip-recovery-apply-tests-when-directory Skip recovery apply tests when directory durability is unsupported

## Description

Restore the native Windows matrix after T-232 added recovery apply tests that
bypass the repository's established directory-durability capability probe.
Recovery must continue to fail closed when the host cannot provide the required
barrier; only tests that require successful durable mutation should skip.

Follow-up derived from T-232-recover-v0-5-transactions-through-one-command's verification or discovery.

## Acceptance

- A1. Service and CLI recovery apply tests probe directory-sync support before
  fabricating retained state and skip visibly when the host reports it unsupported.
- A2. Recovery previews and refusal tests that do not require successful durable
  mutation continue to run on every native host.
- A3. Runtime recovery behavior and unsupported-filesystem classification remain
  unchanged.
- A4. Linux tests, cross-compilation, and the native Windows CI matrix pass.

## Verification Notes

- Run the focused recovery suites, `go vet ./...`, `go test ./...`, and
  `task build:cross` locally; use the post-push Windows matrix as native evidence.

## Implementation Notes

- 2026-08-17T14:58:20Z: Added the established directory-sync capability probe to successful service and CLI recovery apply cases, preserving native refusal coverage and runtime fail-closed behavior while allowing unsupported Windows filesystems to skip visibly.
- 2026-08-17T14:58:27Z: verification pass
