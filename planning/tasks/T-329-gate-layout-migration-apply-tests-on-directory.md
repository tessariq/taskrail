---
id: T-329-gate-layout-migration-apply-tests-on-directory
title: Gate layout migration apply tests on directory durability
status: completed
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-280-publish-layout-2-through-the-durable-migration
updated_at: "2026-08-17T18:05:17Z"
---

# T-329-gate-layout-migration-apply-tests-on-directory Gate layout migration apply tests on directory durability

## Description

Restore the native Windows matrix after T-280 added layout-migration tests that
require successful durable directory barriers. Preserve runtime fail-closed
behavior and continue running preview/refusal coverage on unsupported hosts.

Follow-up derived from T-280-publish-layout-2-through-the-durable-migration's verification or discovery.

## Acceptance

- A1. Layout-migration service and CLI tests that require successful durable
  publication probe directory-sync support and skip visibly when unsupported.
- A2. Preview, input-gate refusal, and runtime unsupported behavior remain active
  on every native host.
- A3. Production migration behavior remains unchanged.
- A4. Focused tests, full tests, cross-builds, and native Windows CI pass.

## Verification Notes

- Run focused layout-migration tests, `go vet ./...`, `go test ./...`, and
  `task build:cross`; use post-push CI as native Windows evidence.

## Implementation Notes

- 2026-08-17T18:05:10Z: Reused the established directory-sync capability probe in only the layout migration and recovery cases that require successful durable publication; preview and refusal coverage remains ungated and runtime behavior is unchanged.
- 2026-08-17T18:05:17Z: verification pass
