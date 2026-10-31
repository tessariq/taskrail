---
id: T-427-validate-each-replayed-parallel-candidate
title: Validate each replayed parallel candidate before committing
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T11:39:09Z"
---

# T-427-validate-each-replayed-parallel-candidate Validate each replayed parallel candidate before committing

## Description

Spec gap in code. The spec says "The coordinator validates that candidate"
before creating its integration commit, and "Every accepted candidate is
repository-valid". On a clean replay the coordinator commits without running
repository validation (internal/taskrail/loop_parallel.go clean path); only one
`Validate()` runs after all candidates. Several failure paths also record no
reason: a failed integration clone, and aggregate pre-checks that return false
without a violation.

Consequence: two dependency-ready tasks that replay cleanly but conflict
semantically (T-001 renames `Old()`, T-002 adds a caller of `Old()` in another
file) produce `batch_pass` with a tree that does not build, unless the
aggregate child happens to run the build. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- Repository validation runs on each candidate before its integration commit; an invalid candidate is `integration_failed` with a coded violation and does not block later independent candidates.
- Integration-clone creation failure and every aggregate pre-check failure add a coded violation to the parallel diagnostic.
- docs/loop.md states that semantic correctness beyond repository validation depends on the caller-supplied aggregate child.

## Verification Notes

- Integration test with a candidate that breaks repository validation after replay.
- Unit tests for the coded violations on clone and aggregate pre-check failures.

## Implementation Notes
