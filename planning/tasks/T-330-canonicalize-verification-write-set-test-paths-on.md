---
id: T-330-canonicalize-verification-write-set-test-paths-on
title: Canonicalize verification write-set test paths on Windows
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-281-protect-verification-and-follow-up-writers
updated_at: "2026-08-17T21:29:18Z"
---

# T-330-canonicalize-verification-write-set-test-paths-on Canonicalize verification write-set test paths on Windows

## Description

Restore native Windows CI after T-281's exact write-set tests compared
`filepath.Rel` output directly with the machine contract's canonical `/` paths.
Canonicalize only the test observation boundary; product path behavior is green.

Follow-up derived from T-281-protect-verification-and-follow-up-writers's verification or discovery.

## Acceptance

- Exact verification write-set and concurrent-addition tests compare canonical
  repository-relative `/` paths on every host.
- Production verification and transaction behavior remains unchanged.
- Focused, full, and native Windows CI tests pass.

## Verification Notes

- Run the focused verification suite, full tests, vet, Windows test compilation,
  and post-push native Windows CI.

## Implementation Notes

- 2026-08-17T21:29:10Z: Canonicalized test-observed changed paths with filepath.ToSlash before comparing exact verification write sets and concurrent additions, leaving product behavior unchanged.
- 2026-08-17T21:29:18Z: verification pass
