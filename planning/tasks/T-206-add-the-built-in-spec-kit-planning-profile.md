---
id: T-206-add-the-built-in-spec-kit-planning-profile
title: Add the built-in Spec Kit planning profile
status: todo
priority: high
spec_ref: specs/v0.7.0.md#spec-kit-profile-version-1
dependencies:
    - T-204-snapshot-planning-sources-with-digest-boundaries
updated_at: "2026-08-05T19:18:03Z"
---

# T-206-add-the-built-in-spec-kit-planning-profile Add the built-in Spec Kit planning profile

## Description

Add the exact built-in `spec-kit` profile version 1 for one Spec Kit-style feature directory. The profile recognizes only the bounded planning outputs named by v0.7, assigns deterministic roles, and passes exact bytes to generic snapshotting. Unsupported generator output, deeper trees, renamed files, and fork-specific structure fail visibly; Taskrail never runs `.specify` scripts or interprets the planning content.

## Acceptance

- A1. Registry resolution of `spec-kit` returns only built-in profile name `spec-kit`, version `1`; there is no local override, plugin, script, matcher configuration, or implicit profile-version negotiation.
- A2. A valid root contains exactly one each of `spec.md`, `plan.md`, and `tasks.md`, assigned roles `specification`, `plan`, and `tasks` respectively.
- A3. The only optional direct files are `research.md`, `data-model.md`, and `quickstart.md`, assigned matching roles `research`, `data-model`, and `quickstart`.
- A4. Optional `contracts/<name>` files receive role `contract` and end only in `.md`, `.json`, `.yaml`, `.yml`, `.graphql`, `.proto`, or `.txt`. Optional `checklists/<name>.md` files receive role `checklist`. Both directories are at most one level deep.
- A5. Each optional child name is one portable component of 1 through 128 ASCII bytes matching `^[A-Za-z0-9][A-Za-z0-9._+-]*$`, with reserved-device, trailing-dot, and case/Unicode alias rules enforced in addition to the extension restrictions.
- A6. Empty optional `contracts` or `checklists` directories are ignored. Missing required files, any other file or directory, deeper optional trees, invalid names/extensions, aliases, templates, memory files, `.specify` content, or generator/fork additions fail with `unknown_layout`; no unknown entry is silently discarded.
- A7. The selected root represents one feature directory, not `.specify`, a template tree, or the repository-wide `specs` parent. Unsupported lookalikes receive an actionable unknown-layout refusal rather than profile guessing.
- A8. The profile assigns roles and cardinality only. It does not execute scripts, read templates or memory, interpret task completion markers, validate contract languages, infer tasks, or parse source prose.
- A9. Every valid required/optional combination feeds the generic descriptor contract in deterministic role/path order, with exact-byte and membership changes reflected in the shared aggregate digest.

## Verification Notes

- A1: Registry and profile-boundary tests should accept exactly `spec-kit`, report version 1 through the public profile result, and reject unknown names or attempts to select/configure another implementation without depending on downstream CLI wiring.
- A2-A5: Table-driven fixtures should cover the minimal shape, every optional direct role, all allowed contract extensions, checklists, multiple entries, valid punctuation, name-length boundaries, and deterministic ordering.
- A6-A7: Negative fixtures should independently remove each required file and add renamed files, invalid extensions/names, nested optional trees, `.specify`, templates, memory, generator output, repository-parent layouts, special entries, ASCII-case aliases, and Unicode aliases. Assert `unknown_layout`, an actionable path, and no partial descriptor.
- A8: Behavioral tests should use arbitrary valid UTF-8 prose, unchecked/checked task markers, and syntactically invalid contract-language content in otherwise valid files; recognition must remain shape-only. Sentinel scripts and hooks must remain unexecuted.
- A9: Integration tests should inspect valid fixtures through the profile-neutral snapshot boundary, compare ordered source lists and aggregate golden vectors, and prove that optional membership, role/path, raw-byte, and CRLF changes affect the descriptor exactly through the generic snapshot contract.
- A1-A9: A temporary-repository service-harness probe should inspect one narrow valid feature directory and one unsupported generator/fork extension, recording success, refusal, and cleanup without running source-system scripts or requiring the downstream root command.

## Implementation Notes
