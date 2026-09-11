---
id: T-399-unindent-starter-spec-heading
title: Unindent the starter spec heading so fresh repos have a coverable area
status: todo
priority: medium
spec_ref: specs/v0.1.0.md#taskrail-init
dependencies: []
updated_at: "2026-09-11T16:21:21Z"
---

# T-399-unindent-starter-spec-heading Unindent the starter spec heading so fresh repos have a coverable area

## Description

`taskrail init` writes the starter `specs/v0.1.0.md` from an indented Go raw
string in `starterSpecV010()` (`internal/taskrail/templates.go`).
`strings.TrimSpace` strips only the outer whitespace, so every line after the
title keeps a leading tab: the file contains `\t## Summary`. The CLI then
disagrees with itself: `spec show --anchors` lists `#summary` and
`task new --spec-ref specs/v0.1.0.md#summary` accepts it, but `coverage`
reports zero coverable areas and rejects `--area summary`, and its error points
back to `spec show --anchors`. The code is identical at v0.4.0, so this is a
long-standing defect rather than a v0.5.0 regression.

Surfaced by the v0.5.0 paired skill evaluation (T-174): the positive paths of
the `taskrail-gap`, `taskrail-decompose`, and task-review cases were
unreachable because a freshly initialized repository has no coverable area.

Outcome: a freshly initialized repository's starter spec has unindented
Markdown, so anchor listing and coverage agree about its headings.

## Acceptance

- A fresh `taskrail init` writes a starter spec whose lines carry no leading
  tab or space indentation.
- `spec show v0.1.0 --anchors` and `coverage` agree on the starter spec's
  headings; any anchor `coverage --area` rejects is not advertised as usable
  by the rejection message.
- Existing repositories are not rewritten: init upgrade leaves an existing
  starter spec's bytes unchanged.

## Verification Notes

- Unit test on `starterSpecV010()` output asserting no indented lines, plus a
  temp-dir init test comparing `spec show --anchors` with `coverage` results.
- Manual sandbox check: fresh init, then `coverage --json` and
  `coverage --area summary`.

## Implementation Notes
