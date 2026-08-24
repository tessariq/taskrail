---
id: T-360-authenticate-delegated-lifecycle-writes-against-loop-grant
title: Authenticate delegated lifecycle writes against the loop grant
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-317-bind-delegated-grants-to-the-owner-s-declared-task
    - T-314-integrate-loop-continuation-and-terminal
updated_at: "2026-08-24T09:24:17Z"
completion_id: "b69619cf7f6442002b85582bcb6a7f17"
last_verification_id: "1f1cb693004b597d61b1de9afee13cd5"
last_verification_result: pass
last_verified_at: "2026-08-24T09:24:17Z"
last_verified_completion_id: "b69619cf7f6442002b85582bcb6a7f17"
---

# T-360-authenticate-delegated-lifecycle-writes-against-loop-grant Authenticate delegated lifecycle writes against the loop grant

## Description

Authenticate every delegated lifecycle subprocess against the same canonical
broad grant that loop ownership issued for its selected task, while continuing
to authorize only each writer's exact narrower command, fields, and paths. The
current writer join incorrectly hashes its command-specific write set as the
parent grant, so one loop child cannot safely perform `start`, `complete`, and a
runtime-destination `verify` with the same task-scoped token.

## Acceptance

- The selected-task loop grant is derived in one shared function as the state
  path, managed tasks-directory prefix, and selected task's verification-artifact
  directory prefix; loop ownership and delegated lifecycle joins use identical
  canonical bytes without adding an environment variable or exposing the grant.
- `start`, `complete`, `block`, and `verify` authenticate one unchanged delegated
  token against that broad grant, then narrow authorization to their exact
  command, task fields, selected task, and concrete transaction write set.
- Another task, another planning/artifact directory, an unselected follow-up, a
  widened command/field set, changed executable identity, or a forged broad grant
  still returns `delegated_write_refused` before mutation.
- Sequential loop behavior remains unchanged, and parallel clone workers can use
  independent clone-local locks without receiving source, sibling, integration,
  task-creation, dependency, or loop-policy authority.
- The exact four delegated environment variables and the closed lock metadata
  schema remain unchanged; documentation and changelog explain only the repaired
  grant authentication semantics.

## Verification Notes

- Start with a failing fixture that issues the production broad grant, runs
  multiple lifecycle writers with narrower concrete paths including a generated
  verification destination, and proves the current HMAC mismatch.
- Extend repolock and lifecycle negative matrices for task/path/command/field and
  executable widening, then run focused sequential loop and delegated writer
  tests, race tests, full tests, vet, build, Taskrail validation, queue/task-body
  checks, and exact-head cross-platform CI.

## Implementation Notes

- 2026-08-24T09:24:06Z: Shared loop grants now authenticate delegated lifecycle writes while preserving narrow per-command authority.
- 2026-08-24T09:24:17Z: verification pass id 1f1cb693004b597d61b1de9afee13cd5 previous none completion b69619cf7f6442002b85582bcb6a7f17
