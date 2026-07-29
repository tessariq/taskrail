---
id: T-151-make-changelog-whitespace-trimming-portable
title: Make changelog whitespace trimming portable
status: todo
priority: low
spec_ref: specs/v0.4.0.md#goals
dependencies:
    - T-149-harden-release-version-and-changelog-notes-guards
updated_at: "2026-07-29T17:30:33Z"
---

# T-151-make-changelog-whitespace-trimming-portable Make changelog whitespace trimming portable

## Description

The changelog extractor trims trailing blank lines with GNU-specific `sed` syntax
(`\s`). BSD/macOS `sed` may not recognize that escape, so a release section whose
body contains only spaces can be treated as non-empty during local release checks.
Use portable whitespace handling while preserving the release-note contract from
`specs/v0.4.0.md#goals`.

Follow-up derived from T-149-harden-release-version-and-changelog-notes-guards's verification or discovery.

## Acceptance

- Changelog trimming uses syntax supported by both GNU and BSD/macOS tooling.
- A release section containing only spaces or tabs is rejected as empty by both
  changelog guard scripts.
- Populated release sections retain their body exactly apart from documented
  leading and trailing blank-line trimming.
- Automated tests cover whitespace-only and populated sections without relying on
  platform-specific behavior.

## Verification Notes

- Review finding from T-149 identified `scripts/changelog-release-notes.sh`'s
  `\s` expression as GNU-specific.

## Implementation Notes
