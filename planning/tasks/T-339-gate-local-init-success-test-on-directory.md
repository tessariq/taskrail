---
id: T-339-gate-local-init-success-test-on-directory
title: Gate local init success test on directory durability
status: completed
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-287-initialize-ignored-local-planning-storage-durably
updated_at: "2026-08-21T08:58:43Z"
completion_id: "3a687a330e1e4d47839a7582f9f6a4fa"
---

# T-339-gate-local-init-success-test-on-directory Gate local init success test on directory durability

## Description

Restore native Windows CI after T-287 added a successful local-initialization
fixture that assumes the host supports durable directory synchronization. Reuse
the established test capability probe while preserving Taskrail's runtime
fail-closed behavior on unsupported filesystems.

Follow-up derived from T-287-initialize-ignored-local-planning-storage-durably's verification or discovery.

## Acceptance

- The successful `init --local` service fixture probes directory durability before
  invoking the durable writer and skips visibly when the host does not support the
  required barrier.
- Local initialization runtime behavior and unsupported-filesystem classification
  remain unchanged; refusal and non-mutating tests continue to run on every host.
- Focused and full tests, vet, cross-compilation, and native Windows CI pass.

## Verification Notes

- Run the focused local-init test, `go vet ./...`, `go test ./...`, and the
  repository cross-compile target locally; use post-push Windows CI as native
  evidence.

## Implementation Notes

- 2026-08-21T08:58:42Z: Gated the local-init success fixture on the established directory-durability capability probe without changing runtime behavior.
- 2026-08-21T08:58:43Z: verification pass
