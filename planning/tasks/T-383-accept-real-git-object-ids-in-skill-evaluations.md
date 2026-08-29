---
id: T-383-accept-real-git-object-ids-in-skill-evaluations
title: Accept real Git object IDs in skill evaluation snapshots
status: completed
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-08-29T16:37:53Z"
completion_id: "405d10f2c711f2543880eb2c68eea065"
last_verification_id: "25220e68c20221a8975221a62eb78545"
last_verification_result: pass
last_verified_at: "2026-08-29T16:37:53Z"
last_verified_completion_id: "405d10f2c711f2543880eb2c68eea065"
---

# T-383-accept-real-git-object-ids-in-skill-evaluations Accept real Git object IDs in skill evaluation snapshots

## Description

Allow the maintainer skill-evaluation runner and strict report decoder to bind
`tested_head` to the full object ID emitted by the clean Git repository under
evaluation. Keep executable, product, skill, fixture, raw, and seal values bound
to lower-case SHA-256 digests.

This removes the release-blocking mismatch where the runner accepts only a
64-character content digest for `tested_head`, although this repository uses a
40-character SHA-1 Git object ID.

## Acceptance

- A run input and canonical report accept a full lower-case 40-character SHA-1
  or 64-character SHA-256 Git object ID as `tested_head`.
- Empty, abbreviated, upper-case, non-hex, or other-length `tested_head` values
  remain invalid.
- Every non-Git digest field still requires exactly 64 lower-case hex
  characters, and stage resumption still rejects a changed tested HEAD.
- A real clean-checkout maintainer evaluation reaches adapter execution with its
  exact `git rev-parse HEAD` value instead of failing input validation.

## Verification Notes

- Cover accepted SHA-1/SHA-256 object IDs and rejected malformed/abbreviated
  values in focused runner and strict report tests.
- Re-run the untouched model-backed evaluation only after deterministic tests,
  full CI, Planning checks, and CodeQL pass on the remediation head.

## Implementation Notes

- 2026-08-29T16:37:53Z: verification pass id 25220e68c20221a8975221a62eb78545 previous none completion 405d10f2c711f2543880eb2c68eea065
