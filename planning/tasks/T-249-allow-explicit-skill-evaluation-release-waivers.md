---
id: T-249-allow-explicit-skill-evaluation-release-waivers
title: Allow explicit skill-evaluation release waivers
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-218-add-maintainer-skill-release-evaluations
updated_at: "2026-08-06T13:46:30Z"
---

# T-249-allow-explicit-skill-evaluation-release-waivers Allow explicit skill-evaluation release waivers

## Description

Allow a maintainer to disclose and accept missing stochastic skill-evaluation
evidence without misrepresenting provider absence or an incomplete run as pass.

## Acceptance

- A1. The committed evaluation summary supports `waived` only with approver,
  reason, unavailable capability, affected cases, residual risk, compensating
  evidence, and follow-up issue or target release.
- A2. Waiver requires all credential-free deterministic checks to pass and cannot
  waive parity, lifecycle, machine API, command, security, or cross-platform gates.
- A3. Release notes disclose waiver; `fail` and `incomplete` remain blockers and raw
  provider transcripts remain ignored.

## Verification Notes

- A1: strict report fixtures cover pass/fail/incomplete/valid and malformed waiver.
- A2: gate tests attempt every forbidden waiver and observe refusal.
- A3: release-summary and ignored-raw-output fixtures prove disclosure and privacy.

## Implementation Notes
