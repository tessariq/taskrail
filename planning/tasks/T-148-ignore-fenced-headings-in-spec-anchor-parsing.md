---
id: T-148-ignore-fenced-headings-in-spec-anchor-parsing
title: Ignore fenced headings in spec anchor parsing
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#spec-version-diff
dependencies:
    - T-113-spec-diff
    - T-139-heading-match-crlf-fences
updated_at: "2026-07-29T13:05:08Z"
---

# T-148-ignore-fenced-headings-in-spec-anchor-parsing Ignore fenced headings in spec anchor parsing

## Description

The shared spec heading scanners treat lines inside fenced code blocks as real
Markdown headings. A documented example can therefore become a valid `spec_ref`, a
coverable area, or a spec-diff fact even though rendered Markdown has no such
heading.

## Acceptance

- `spec show --anchors`, `validate` spec-ref resolution, `coverage`,
  `coverage --gaps`, and `spec diff` ignore ATX-looking lines inside backtick and
  tilde fences.
- Indented and language-tagged opening fences, matching close lengths, CRLF input,
  and unclosed fences follow the repository's existing Markdown fence policy.
- Real headings immediately before and after a fence remain discoverable in order.
- Tests cover the shared parser so command surfaces cannot drift.

## Verification Notes

- T-140 sandbox evidence showed `spec show --anchors` listing
  `#not-a-real-area` from a fenced `### Not A Real Area` example.

## Implementation Notes

- Reuse the task-body fence matching semantics hardened in T-139 where practical.
