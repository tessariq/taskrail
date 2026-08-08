---
id: T-249-allow-explicit-skill-evaluation-release-waivers
title: Allow explicit skill-evaluation release waivers
status: todo
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies:
    - T-218-add-maintainer-skill-release-evaluations
updated_at: "2026-08-08T08:40:49Z"
---

# T-249-allow-explicit-skill-evaluation-release-waivers Allow explicit skill-evaluation release waivers

## Description

Allow a maintainer to disclose and accept missing stochastic skill-evaluation
evidence without misrepresenting provider absence or an incomplete run as pass.

## Acceptance

- A1. The committed evaluation summary supports `waived` only with approver,
  reason, unavailable capability, affected skills/cases, residual risk, compensating
  evidence, and follow-up issue or target release. It uses the exact schema-v1
  nullable waiver union; pass/fail/incomplete require
  null waiver, and unknown, missing, unsorted, duplicate, or outcome-inconsistent
  fields fail strict decoding. A valid waiver covers exactly every missing or
  incomplete case and their skill set, cannot cover a failed case/check, and does
  not self-authorize its approver; the release operator verifies authority against
  repository governance outside the report. Strict fixtures do not claim to
  mechanically certify that human authority; T-174 records its disposition.
- A1 exclusively owns non-null waiver decoding, exact `waived` precedence, affected
  incomplete arm/case/skill coverage, and malformed-waiver fixtures; T-218 owns the
  base report and null waiver slot.
- A2. Waiver requires all credential-free deterministic checks to pass and cannot
  waive parity, lifecycle, machine API, command, security, or cross-platform gates.
- A3. Release notes disclose waiver; `fail` and `incomplete` remain blockers and raw
  provider transcripts remain ignored.

## Verification Notes

- A1: strict fixtures cover valid and malformed non-null waiver, including field
  order, non-empty scalar syntax, nullability, array sorting, exact incomplete-set
  coverage, and every outcome/waiver mismatch.
- A2: gate tests attempt every forbidden waiver and observe refusal.
- A3: release-summary and ignored-raw-output fixtures prove disclosure and privacy.

## Implementation Notes
