---
id: T-386-move-skill-eval-report-schema-out-of-spec-prose
title: Move the skill-evaluation report schema out of spec prose
status: todo
priority: medium
spec_ref: specs/v0.6.0.md#behavioral-and-release-acceptance
dependencies: []
updated_at: "2026-09-08T07:27:34Z"
---

# T-386-move-skill-eval-report-schema-out-of-spec-prose Move the skill-evaluation report schema out of spec prose

## Description

`specs/v0.5.0.md` spends roughly 160 lines (the `Maintainer Skill Release
Evaluations` subsection) prose-specifying the byte layout of the committed
skill-evaluation `report.json`: field order, indentation, trailing LF, digest
casing, sort keys, and nullability. That is longer than the whole `v0.1.0` spec
and it makes every report-schema adjustment a spec change, which in turn
restarts the release gate. The behaviour users depend on is the intent (safe
summary, digest-only raw references, paired outcomes, human review), not the
byte layout.

The independently meaningful outcome is that the report contract lives in one
Go schema plus a committed golden fixture, while the spec keeps only the intent
and points at the code-owned contract, so schema maintenance no longer churns
the normative product spec.

## Acceptance

- The report schema is defined once in Go (`internal/taskrail`) with a committed
  golden `report.json` fixture that the encoder must reproduce byte-for-byte and
  the decoder must accept; the strict validation rules the spec currently spells
  out move into tests against that fixture, with no relaxation of any rule.
- `specs/v0.5.0.md#maintainer-skill-release-evaluations` is reduced to intent
  (what the report must guarantee and where it lives) plus a reference to the
  code-owned contract; the released v0.5.0 spec bytes are only edited if this
  lands before the tag, otherwise the reduction happens in the `v0.6.0` spec
  as an explicit supersession and `v0.5.0` stays frozen.
- `docs/workflow/skill-evaluation.md` names the schema location and the golden
  fixture instead of restating fields.
- No committed `planning/reviews/skill-evals/` report changes bytes; the existing
  report remains valid under the code-owned schema.

## Verification Notes

- Encode the golden fixture from a constructed report and diff against the
  committed bytes; decode the committed bytes and assert round-trip equality.
- Run `task check:skills`, `go test ./internal/taskrail -run 'SkillEval'`, and
  `taskrail validate` after the spec edit; confirm `coverage` still reports the
  subsection covered.
- Cheapest evidence is the test output plus the spec diff line count.

## Implementation Notes
