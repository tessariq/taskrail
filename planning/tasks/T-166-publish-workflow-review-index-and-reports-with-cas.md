---
id: T-166-publish-workflow-review-index-and-reports-with-cas
title: Publish workflow review index and reports with CAS
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-165-maintain-bounded-workflow-adversarial-review
updated_at: "2026-08-04T21:32:13Z"
---

# T-166-publish-workflow-review-index-and-reports-with-cas Publish workflow review index and reports with CAS

## Description

Add the workflow-review publisher that turns reviewed index/report proposals into
one race-safe no-clobber publication without losing another reviewer's findings.

## Acceptance

- The command requires layout 2, joins the shared writer discipline, then acquires
  repository and review locks in one documented global order.
- It rechecks expected HEAD, spec/product/index digests, path boundaries, strict
  schemas, caps, and absent report destination immediately before publication.
- The report publishes no-clobber and the index CAS-replaces atomically as one
  outcome; conflict or interruption exposes neither final file and reports exact
  recovery.
- Concurrent publishers cannot reuse IDs, clobber reports, deadlock, or lose rows;
  symlink/reparse and sibling/input aliases are rejected.
- A human must commit or discard the allowed index/report diff before another
  clean review.

## Verification Notes

- Map criteria to lock-order concurrency tests, stale snapshot cases, no-follow
  path swaps, ID races, interrupted first/second publication, rollback
  observations, and clean handoff diffs.
- Run two simultaneous publisher attempts and prove exactly one complete
  publication.

## Implementation Notes
