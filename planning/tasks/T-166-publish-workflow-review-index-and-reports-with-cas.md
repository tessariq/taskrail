---
id: T-166-publish-workflow-review-index-and-reports-with-cas
title: Publish workflow review index and reports with CAS
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-165-maintain-bounded-workflow-adversarial-review
    - T-215-add-the-generic-review-artifact-publisher
    - T-232-recover-v0-5-transactions-through-one-command
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
updated_at: "2026-08-04T21:32:13Z"
---

# T-166-publish-workflow-review-index-and-reports-with-cas Publish workflow review index and reports with CAS

## Description

Integrate one immutable workflow report with the generic publisher, mechanically
derive canonical memory from the prior index and report, and publish the pair
race-safely without losing another reviewer's findings.

## Acceptance

- The workflow type requires layout 2 and uses only the shared repository mutation
  lock and durable recovery protocol; it introduces no second review lock.
- Its capability and write set cover only the review index and report destination;
  task fields, including `loop_policy` and `loop_reason`, are explicitly excluded.
- It rechecks expected HEAD, recorded-tree product/spec/prior-index digests, path
  boundaries, strict report schema, current workflow prompt source/template,
  file/index caps, and absent report destination.
- Taskrail applies exact transition rules to derive candidate `INDEX.json`; the
  agent never supplies candidate index bytes, and unexplained prior rows persist.
- Report no-clobber plus index CAS is one durable logical outcome. Interruption may
  leave fenced physical bytes, but Taskrail exposes no lone logical output and the
  shared recovery command derives the safe action. Prompt/config bytes enter the
  pre-fence read set; the derived index never duplicates report prompt metadata.
- Concurrent publishers cannot reuse IDs, clobber reports, deadlock, or lose rows;
  symlink/reparse and sibling/input aliases are rejected.
- A human must commit or discard the allowed index/report diff before another
  clean review.

## Verification Notes

- Map criteria to lock-order concurrency tests, stale snapshot cases, no-follow
  path swaps, prompt replacement races, ID races, interrupted first/second
  publication, rollback observations, and clean handoff diffs.
- Run two simultaneous publisher attempts and prove exactly one complete
  publication.

## Implementation Notes
