---
id: T-409-reachable-skill-eval-scenarios
title: Seed skill-eval fixtures so each case prompt is reachable
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-11-01T20:57:18Z"
completion_id: "04c1650a6ad144938388881cd1fd6c54"
last_verification_id: "6845a871bf02632fc846c39f6bf76c31"
last_verification_result: pass
last_verified_at: "2026-11-01T20:57:18Z"
last_verified_completion_id: "04c1650a6ad144938388881cd1fd6c54"
---

# T-409-reachable-skill-eval-scenarios Seed skill-eval fixtures so each case prompt is reachable

## Description

Many registered cases ask for flows their fixture cannot reach, so arms only
exercise refusals: recovery and repair seed no drift; autonomous-verify seeds
no invalid evidence; retrofit runs on an already-initialized repository and
always gets `destination_exists`; loop cases set no loop policy, so the dry run
is always `action: none`; sdd-handoff seeds no OpenSpec or Spec Kit artifacts;
decompose seeds no published post-spec review; spec seeds no target spec; gap
and decompose have zero coverable areas (T-399). The
`taskrail-retrofit-committed` baseline invented "reviewed" notes to fill the gap.

Outcome: every case's prompt claims only what its setup establishes.

## Acceptance

- For each registered case, the positive or recovery condition its prompt names
  exists after setup, or the prompt is narrowed to what does.
- Registry validation mechanically checks what it can (setup establishes the
  named condition).

## Verification Notes

- Registry tests; a model-backed replay of changed cases before the next
  release evaluation. Coordinate with T-399.

## Implementation Notes

- 2026-11-01T20:57:09Z: All 32 registered prompts now match disposable setup conditions, with explicit loop allowance, no invented review evidence, and deterministic reachability checks including T399 starter coverage. Simplification and bounded independent review closed C1-C5; unfiltered go test ./..., vet, formatting, parity, task-body hygiene, and validate passed. No model/provider-backed evaluation ran. T174 retains the outstanding changed-case model-backed replay obligation before the next release evaluation and owns the final snapshot/report; deterministic tests are not model evidence. Delivery remains held for independent integrated acceptance of the exact candidate.
- 2026-11-01T20:57:18Z: verification pass id 6845a871bf02632fc846c39f6bf76c31 previous none completion 04c1650a6ad144938388881cd1fd6c54
