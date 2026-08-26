---
id: T-164-orchestrate-adversarial-spec-to-task-decomposition
title: Orchestrate adversarial spec-to-task decomposition
status: completed
priority: high
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies:
    - T-163-validate-and-apply-importdraft-v2-transactionally
    - T-300-bind-decomposition-publication-to-resolved-prompts
    - T-304-align-imported-and-decomposed-task-bodies
updated_at: "2026-08-26T12:54:09Z"
completion_id: "f06d576da401f1998befc4846a85c311"
last_verification_id: "faad48f3b5798b6f26d909a4dd8da1f7"
last_verification_result: pass
last_verified_at: "2026-08-26T12:54:09Z"
last_verified_completion_id: "f06d576da401f1998befc4846a85c311"
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
- Decomposition applies the T-251 rubric across the bundle: split independently
  valuable outcomes, merge fragments that cannot deliver value alone, preserve
  coherent acceptance boundaries, and assign integration behavior and ownership
  explicitly rather than leaving cross-task assembly implicit.
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
  duplicate-task detection, oversized/fragmented bundle findings, explicit
  integration ownership, loop-policy escalation rejection, context metadata,
  subject/template digest mutations, prompt changes between passes, pass-limit
  stops and explicitly initiated restarts, post-apply validation, coverage, and
  tracked handoff diffs.
- Run one first-pass-clean and one materially revised two-pass sandbox
  decomposition end to end.

## Implementation Notes

- 2026-08-26T12:53:56Z: Strengthened digest-bound decomposition authoring and adversarial review guidance, including inactive-spec handoff safety.
- 2026-08-26T12:54:09Z: verification pass id faad48f3b5798b6f26d909a4dd8da1f7 previous none completion f06d576da401f1998befc4846a85c311
