---
id: T-218-add-maintainer-skill-release-evaluations
title: Register deterministic skill evaluations
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-254-make-every-packaged-skill-storage-neutral
updated_at: "2026-08-08T08:40:49Z"
---

# T-218-add-maintainer-skill-release-evaluations Register deterministic skill evaluations

## Description

Define and deterministically validate the complete maintainer-owned behavioral
evaluation registry for every shipped skill. Provider execution and paired report
construction remain T-307.

## Acceptance

- A1. Eval definitions live outside the embedded skill tree; each strict case binds
  stable prompt, expected observation, assertions, human-review questions, storage
  mode, baseline requirement, and complete fixture bytes.
- A2. The exact checked registry represents every globally unique case ID, binds every
  shipped skill in committed and local storage, requires exact prompt/expected-observation/
  assertions/human-review-question metadata, and rejects empty, duplicate,
  mismatched, single-storage, or missing-skill coverage.
- A3. Cases include realistic positive, negative, recovery, and boundary behavior;
  local fixtures include decoy logical paths and visible Git provenance assertions
  that distinguish caller authority from managed planning content.
- A4. Deterministic registry and package/skill/fixture tree digests use the
  normative domain-separated framing, fixed v0.4.0 baseline commit, and clean
  candidate snapshot without invoking an agent or writing raw/report evidence.
- A5. Registry assets are absent from embedded/install output and coexist with the
  credential-free skill contract/parity checks rather than replacing them.

## Verification Notes

- A1-A3: registry mutation tests cover strict fields, path/name agreement,
  duplicate IDs, baseline classification, authored arrays, and complete
  committed/local coverage for every packaged skill.
- A4/A5: digest goldens bind candidate, baseline, skill, and fixture trees; install
  snapshots prove eval definitions, credentials, transcripts, and runners are not
  packaged.

## Implementation Notes
