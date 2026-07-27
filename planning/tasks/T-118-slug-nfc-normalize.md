---
id: T-118-slug-nfc-normalize
title: Normalize slug input with x/text before transliterating
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#slugged-task-creation
dependencies:
    - T-115-slug-nfd-folding
updated_at: "2026-07-27T13:26:30Z"
---

# T-118-slug-nfc-normalize Normalize slug input with x/text before transliterating

## Description

T-115 closed the NFD hole with a hand-rolled fold: a replacer for the three
decomposed umlauts plus a pass that strips leftover combining marks. That covers
the common cases but is a partial reimplementation of Unicode normalization — it
only recognizes a combining mark sitting immediately after its base letter, and it
enumerates by hand the decomposed sequences it knows about, so any accented letter
outside the hand-written list still degrades differently depending on whether the
operator's keyboard emitted it composed or decomposed.

Worse, the gap runs the other way too: a **precomposed** letter outside the
hand-written Latin-1 table (`ẅ`, `š`, `ā`, Vietnamese `ế`) matches no entry at all
and is dropped whole by the non-alphanumeric collapse, so a title of such letters
slugifies to nothing.

Replace the hand-rolled half with real normalization: decompose the input with
`golang.org/x/text/unicode/norm` (NFD) and strip the combining marks, so **every**
accented letter folds to its base regardless of script or input spelling. The
hand-written table then shrinks to the letters that genuinely have no canonical
decomposition (`ß`, `æ`, `œ`, `ø`, `þ`, `ð`) — the fold gets both more correct and
smaller.

Note NFD, not NFC: composing would map decomposed input onto precomposed runes the
Latin-1 table still has to enumerate one by one, which is the very list this task
exists to stop maintaining.

This adds the repository's first non-CLI runtime dependency. `x/text` is the
Go project's own Unicode module, and hand-maintaining normalization tables is the
alternative; the trade was approved explicitly rather than assumed. AGENTS.md's
standard-library preference stands for everything else.

## Acceptance

- `golang.org/x/text` is a direct dependency in `go.mod`, with `go.sum` updated and
  `go mod tidy` clean.
- `slugify` normalizes to NFD before transliterating, and the precomposed
  accented-letter entries are **deleted** from the transliteration table: what
  remains is only the letters with no canonical decomposition. The German umlaut
  expansion keeps its own decomposed-form replacer, since dropping marks alone
  would yield `u` where German convention wants `ue`.
- Every T-115 case still passes unchanged: NFD and precomposed spellings of
  `Über Fußball`, `Café Niño Français`, `Ärger Öl` and `Ångström` agree, and a
  stray combining mark still folds away.
- Coverage extends past the hand-written list in **both** spellings: letters that
  were never in the table (`ẅ`, `š`, `ā`, Vietnamese `ế`) fold to their base letter
  whether they arrive precomposed or decomposed. Before this task the precomposed
  spellings slugified to empty.
- The residual limitation is characterized by a test rather than left implicit:
  stacked marks at equal combining class are not reordered by NFC, so
  `u`+U+0301+U+0308 folds to the base letter, not to `ue`. Assert the actual
  behavior so it is documented and cannot change silently.
- No behavior change for ASCII input: the existing slug corpus is unaffected.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-27T13:26:30Z: verification pass
