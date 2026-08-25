---
id: T-300-bind-decomposition-publication-to-resolved-prompts
title: Bind decomposition publication to resolved prompts
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
    - T-293-publish-decomposition-review-bundles-safely
updated_at: "2026-08-25T19:28:56Z"
completion_id: "ac64541954721e93d0603cf13f0d42cf"
last_verification_id: "153bcd72a9a96c3f27c4949429af7ccf"
last_verification_result: pass
last_verified_at: "2026-08-25T19:28:56Z"
last_verified_completion_id: "ac64541954721e93d0603cf13f0d42cf"
---

# T-300-bind-decomposition-publication-to-resolved-prompts Bind decomposition publication to resolved prompts

## Description

Bind each decomposition adversarial review pass to the exact current
`task-decomposition-adversarial` template when its final bundle is published.

## Acceptance

- A1. Every included review pass requires prompt ID
  `task-decomposition-adversarial`, contract `v1`, exact template digest, and
  effective source; draft, trace, and manifest do not duplicate prompt metadata.
- A2. One-pass and two-pass bundles validate role/binding shape before active
  resolution and apply the specified malformed, invalid-replacement, and stale
  source/template precedence.
- A3. Preview/apply snapshot and final-recheck every included prompt/config
  resolution with the decomposition bundle; drift in either pass publishes nothing.
- A4. Published bundles remain valid inputs after later prompt changes and make no
  claim that an external fresh context received or followed the template.

## Verification Notes

- A1-A3: one/two-pass built-in and committed/local replacement matrices cover role,
  contract, source, digest, invalid replacement, source transition, config race,
  and final-recheck failures with absent destinations.
- A4: historical read/import fixtures retain exact published bytes after prompt
  changes; wording checks reject delivery and independence attestations.

## Implementation Notes

- 2026-08-25T19:28:43Z: Added decomposition prompt-binding matrices and final commit recheck coverage.
- 2026-08-25T19:28:56Z: verification pass id 153bcd72a9a96c3f27c4949429af7ccf previous none completion ac64541954721e93d0603cf13f0d42cf
