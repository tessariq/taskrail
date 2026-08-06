---
id: T-181-detect-durable-physical-task-path-references
title: Detect durable physical task path references
status: todo
priority: high
spec_ref: specs/v0.6.0.md#durable-path-reference-preflight
dependencies:
    - T-179-resolve-stable-task-references-across-every
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-180-make-semantic-publication-durably-transactional
updated_at: "2026-08-04T23:06:23Z"
---

# T-181-detect-durable-physical-task-path-references Detect durable physical task path references

## Description

Build the layout-independent fail-closed scanner core used by migration and
later archive, restore, rename, and validation integrations without rewriting
text.

## Acceptance

- Git scanning enumerates tracked/index candidates, rejects flags/disagreement,
  and classifies UTF-8, BOM UTF-16, symlink targets, binary, gitlink, encoding,
  and filter scope exactly.
- Non-Git scanning enumerates canonical regular root files with exact exclusions
  and incomplete-scope warnings.
- Local-mode Git scanning combines clean tracked text with every managed-overlay
  semantic file, translates physical paths to logical paths, and refuses external
  untracked or incomplete overlay scope.
- Matching uses host-independent conservative ASCII case fold for
  slash/backslash, relative, anchor/suffix, and link destinations decoded
  exactly once; malformed or decoded separator/control ambiguity refuses, while
  percent remains non-recursive.
- Blockers expose bounded escaped path/line/Unicode-column/excerpt
  deterministically plus scan_complete/unsupported_paths; source/candidate
  roots and self/inverse refs need no active layout 3.
- Core is read-only, never rewrites/tombstones/redirects, and leaves bare
  ID-like prose outside matching.

## Verification Notes

- Map criteria to
  raw/once-encoded/%252F/malformed/relative/case/Windows/control/longer names,
  encodings, gitlinks/filters/index flags, symlinks, non-Git scope, and source
  candidates.
- Prove zero writes and exact remediation on every blocker/incomplete case.

## Implementation Notes
