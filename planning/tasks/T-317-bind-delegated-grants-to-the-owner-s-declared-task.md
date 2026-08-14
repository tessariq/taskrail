---
id: T-317-bind-delegated-grants-to-the-owner-s-declared-task
title: Bind delegated grants to the owner's declared task and write set
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
updated_at: "2026-08-14T13:11:43Z"
---

# T-317-bind-delegated-grants-to-the-owner-s-declared-task Bind delegated grants to the owner's declared task and write set

## Description

A delegated join currently declares its own selected task and write set; repolock only proves they are non-empty and within the fixed delegated command/field bound, because the normative lock metadata set is closed and records no capability. Bind the grant instead, for example by deriving the recorded delegation digest from the token plus the canonical bound, so a child presenting a wider selected task or write set fails the digest comparison. Covers specs/v0.5.0.md#repository-discovery-locking-and-recovery and #cross-platform-autonomous-loop.

## Acceptance

- The follow-up issue described by verification is resolved.
- Verification evidence is updated.

## Verification Notes

- Re-run task-scoped verification after implementing the fix.

## Implementation Notes

- 2026-08-14T13:11:31Z: Bound each delegation digest to the owner's canonical selected task and write set with HMAC-SHA256; joins authenticate that grant before allowing narrower capabilities, with repository, storage, executable, fixed metadata, and nested narrowing preserved.
- 2026-08-14T13:11:43Z: verification pass
