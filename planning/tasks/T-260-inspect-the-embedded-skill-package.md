---
id: T-260-inspect-the-embedded-skill-package
title: Inspect the embedded skill package
status: todo
priority: high
spec_ref: specs/v0.6.0.md#embedded-skill-inspection-and-agent-mode
dependencies:
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-259-add-explicit-agent-mode-and-structured-help
updated_at: "2026-08-08T11:20:05Z"
---

# T-260-inspect-the-embedded-skill-package Inspect the embedded skill package

## Description

Expose read-only `skill list` and `skill show` over the exact package embedded in
the running binary, independent of repository initialization or installed copies.

## Acceptance

- A bidirectional manifest rejects registered-missing and embedded-unregistered
  skills; list output is deterministic and reports exact metadata, files, sizes,
  and digests.
- Show validates the skill and nested relative file selector, emits byte-exact
  text by default, and emits the exact schema-version-2 content result under JSON
  or agent mode.
- Unknown skill/file and malformed path errors list actionable valid choices
  without traversing outside the embedded subtree.
- Neither command discovers, warns about, installs, refreshes, compares, or
  evaluates materialized skills, and both work from outside a Taskrail repository.

## Verification Notes

- Cover manifest completeness, ordering, digest vectors, nested resources, path
  attacks, exact bytes, JSON escaping, invalid embedded metadata, and installed-
  copy isolation with unit and packaged smoke tests.

## Implementation Notes
