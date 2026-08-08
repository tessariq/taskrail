---
id: T-164-orchestrate-adversarial-spec-to-task-decomposition
title: Orchestrate adversarial spec-to-task decomposition
status: todo
priority: high
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies:
    - T-163-validate-and-apply-importdraft-v2-transactionally
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
updated_at: "2026-08-04T21:32:13Z"
---

# T-164-orchestrate-adversarial-spec-to-task-decomposition Orchestrate adversarial spec-to-task decomposition

## Description

Upgrade the decomposition prompt and skill to author a complete trace/draft pair,
obtain a fresh adversarial review of exact bytes, resolve findings within the
two-pass bound, and hand an approved manifest to the v2 writer.

## Acceptance

- Authoring first validates the final spec-review manifest; active specs use
  coverage gaps, inactive specs use discovered anchors plus existing-task
  duplicate inspection.
- Every normative requirement has one quote-or-lines trace source and task/no-task
  disposition; every trace/draft key is bidirectionally valid and every task body
  follows the shared authoring contract.
- Decomposition and review artifacts cannot authorize unattended execution;
  proposed tasks omit `loop_policy` and `loop_reason` and arrive implicitly held.
- Reviews record fresh-process/context or explicitly accepted same-context mode
  and exact prompt/spec/draft/trace bindings; high/medium cannot defer and final
  bytes receive review.
- Material changes require pass 2; material changes after pass 2 invalidate the
  session and stop instead of applying unreviewed bytes. Another session starts
  only when the human explicitly initiates it. Prompt drift between a pass and
  publication likewise abandons the stale session rather than editing metadata.
- Human approval binds all final files; apply is followed by validate and the
  relevant coverage report. Final session artifacts publish through the generic
  review command before import; abandoned sessions are removed.

## Verification Notes

- Map criteria to active/inactive fixture sessions, requirement coverage oracles,
  duplicate-task detection, loop-policy escalation rejection, context metadata,
  subject/template digest mutations, prompt changes between passes, pass-limit
  stops and explicitly initiated restarts, post-apply validation, coverage, and
  tracked handoff diffs.
- Run one first-pass-clean and one materially revised two-pass sandbox
  decomposition end to end.

## Implementation Notes
