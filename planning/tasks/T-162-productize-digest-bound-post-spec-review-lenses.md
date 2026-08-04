---
id: T-162-productize-digest-bound-post-spec-review-lenses
title: Productize digest-bound post-spec review lenses
status: todo
priority: high
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies:
    - T-159-add-a-versioned-workflow-prompt-catalog
updated_at: "2026-08-04T21:32:13Z"
---

# T-162-productize-digest-bound-post-spec-review-lenses Productize digest-bound post-spec review lenses

## Description

Turn consistency, gaps, additions, and adversarial spec review into independent
prompt and skill handoffs with strict JSON artifacts and one final digest-bound
disposition manifest.

## Acceptance

- Separate contexts independently emit fixed lens filenames with the exact
  schema-v1 fields, severity meanings, evidence, scope, disposition, session
  identity, selected spec path, and lower-case digest without receiving earlier
  conclusions as facts.
- The final manifest binds one final spec digest, every lens path/file
  digest/spec digest, and exactly one disposition per finding; strict decoding
  rejects unknown, null, duplicate, missing, or malformed data.
- Accepted findings name resulting headings, rejected findings explain why,
  deferred findings name a future version, and unresolved high/medium findings
  forbid decomposition.
- Any spec byte edit stales all four lens reports and requires all four rerun
  against the final digest; additions cannot silently expand scope.
- Caller outputs use canonical no-follow, no-alias, absent-destination,
  same-directory atomic no-clobber publication; lenses remain advisory and cannot
  invoke semantic writers or gate validate.

## Verification Notes

- Map criteria to exact-schema fixtures, independent-context metadata,
  publication race/alias cases, stale digest detection, complete dispositions,
  and forbidden-writer prompt mutations.
- Run all four lenses, edit one spec byte, prove the mixed snapshot cannot
  manifest, then rerun all four and approve final bytes.

## Implementation Notes
