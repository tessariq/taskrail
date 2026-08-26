---
id: T-371-gate-implicit-local-bootstrap-tests-by-durability
title: Gate implicit local bootstrap tests by durability capability
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-245-cover-the-complete-implicit-local-bootstrap-matrix
updated_at: "2026-08-26T13:40:05Z"
completion_id: "0287f337aeb984a740add85d1f4c4070"
last_verification_id: "1e92ded682cc91b8ed18e506b357559a"
last_verification_result: pass
last_verified_at: "2026-08-26T13:40:05Z"
last_verified_completion_id: "0287f337aeb984a740add85d1f4c4070"
---

# T-371-gate-implicit-local-bootstrap-tests-by-durability Gate implicit local bootstrap tests by durability capability

## Description

Keep the implicit local-bootstrap success-path command matrix portable across
filesystems that do not support Taskrail's required parent-directory durability
barrier, without weakening the production refusal.

## Acceptance

- Implicit local-bootstrap tests execute normally when directory synchronization
  is supported and skip before mutation when the test filesystem cannot provide
  the required durability barrier.
- Production local bootstrap continues to fail closed when directory durability
  is unsupported.

## Verification Notes

- Run the focused command tests and the full Go suite locally, then require the
  native Windows CI job to pass at the exact delivered commit.

## Implementation Notes

- 2026-08-26T13:40:05Z: Gate implicit local-bootstrap mutating tests on filesystem directory-sync capability while retaining read-only coverage and production fail-closed behavior.
- 2026-08-26T13:40:05Z: verification pass id 1e92ded682cc91b8ed18e506b357559a previous none completion 0287f337aeb984a740add85d1f4c4070
