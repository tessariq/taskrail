---
id: T-139-heading-match-crlf-fences
title: Make task-body heading matching robust to CRLF and fenced code blocks
status: todo
priority: low
spec_ref: specs/v0.2.0.md#no-local-paths-in-task-notes
dependencies:
    - T-135-duplicate-heading-guard-all-sections
updated_at: "2026-07-29T10:37:20Z"
---

# T-139-heading-match-crlf-fences Make task-body heading matching robust to CRLF and fenced code blocks

## Description

Heading matching over a task body is line-based and trims only `" \t"`
(`hasImplementationNotesHeading`, `internal/taskrail/tasks.go`). Two consequences
surfaced while reviewing T-135:

- CRLF: a `\r`-terminated `## Implementation Notes` line is not recognised, so the
  note writer would scaffold a second heading into a CRLF-authored task file. T-135
  hardened only the corpus *detector* (it trims `\r`); the writer still does not.
- Fenced code blocks: a scaffold heading quoted inside a ``` fence counts as a real
  section. The corpus guard now scopes to four generic headings (`## Description`,
  `## Acceptance`, ...), so a task body that illustrates the scaffold in a fence
  would fail `task check:task-bodies` as a false positive.

Neither is currently triggered by the corpus; both were rated low/medium in review.

Follow-up derived from T-135-duplicate-heading-guard-all-sections's verification or discovery.

## Acceptance

- `hasImplementationNotesHeading` recognises a CRLF-terminated heading, so `verify`
  and `block` append to the existing section instead of scaffolding a duplicate.
- The corpus duplicate guard ignores scaffold headings inside fenced code blocks,
  while keeping the whole-line match for real headings.
- Focused tests cover both cases; the existing corpus and fixture assertions stay green.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
