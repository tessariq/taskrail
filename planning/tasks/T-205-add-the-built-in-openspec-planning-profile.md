---
id: T-205-add-the-built-in-openspec-planning-profile
title: Add the built-in OpenSpec planning profile
status: todo
priority: high
spec_ref: specs/v0.7.0.md#openspec-profile-version-1
dependencies:
    - T-204-snapshot-planning-sources-with-digest-boundaries
updated_at: "2026-08-05T19:17:59Z"
---

# T-205-add-the-built-in-openspec-planning-profile Add the built-in OpenSpec planning profile

## Description

Add the exact built-in `openspec` profile version 1 for one OpenSpec-style change directory. The profile recognizes only the bounded v0.7 interchange shape, assigns deterministic roles to its files, and hands their untouched bytes to generic snapshotting. Anything outside that shape fails as unknown layout; the binary does not parse OpenSpec prose or imply compatibility with other releases, forks, archives, or repository-wide layouts.

## Acceptance

- A1. Registry resolution of `openspec` returns only built-in profile name `openspec`, version `1`; there is no local override, plugin, matcher configuration, or implicit profile-version negotiation.
- A2. A valid root contains exactly one `proposal.md` with role `proposal`, one `tasks.md` with role `tasks`, optional `design.md` with role `design`, and one or more `specs/<capability>/spec.md` files with role `requirement`.
- A3. `<capability>` matches `^[a-z0-9][a-z0-9-]{0,62}$`, and `specs` has exactly one capability-directory level. Required canonical lower-case names are matched case-sensitively on every host.
- A4. Required-file absence, zero requirement files, extra direct files, extra or nested directories, metadata, alternate requirement filenames, archived changes, generated artifacts, case/Unicode aliases, and any otherwise unrecognized entry fail with `unknown_layout`; no entry is ignored to make a near match pass.
- A5. The selected root represents one change directory, not the top-level `openspec` directory or an archived changes collection. A lookalike unsupported shape receives an actionable unknown-layout refusal rather than profile guessing.
- A6. The profile assigns roles and cardinality only. It does not parse proposal sections, scenarios, capability semantics, task checkboxes/completion state, upstream configuration, or source prose, and it never executes OpenSpec tooling.
- A7. Valid required/optional combinations feed the generic descriptor contract with canonical paths and deterministic role/path ordering; changing optional membership or any exact source byte changes the aggregate through the shared snapshot boundary.

## Verification Notes

- A1: Registry and profile-boundary tests should accept exactly `openspec`, report version 1 through the public profile result, and reject unknown names or attempts to select/configure another implementation without depending on downstream CLI wiring.
- A2-A3: Table-driven profile fixtures should cover the minimal valid shape, optional design, multiple valid capabilities, capability length boundaries, and deterministic requirement ordering.
- A4-A5: Negative fixtures should independently remove each required role and add each forbidden file/directory shape, including zero requirements, nested capability trees, archive-like collections, top-level repository layouts, renamed files, ASCII-case variants, and Unicode aliases. Assert `unknown_layout` and identify the offending or missing path without returning a partial descriptor.
- A6: Behavioral tests should place syntactically unusual or contradictory UTF-8 prose and arbitrary task checkbox states in otherwise valid files and prove recognition depends only on shape and bytes. Sentinel scripts/hooks must remain unexecuted.
- A7: Integration tests should pass valid fixtures through the profile-neutral snapshot boundary, compare ordered source lists and aggregate golden vectors, then change design membership and one raw byte to prove digest sensitivity.
- A1-A7: A temporary-repository service-harness probe should inspect one narrow valid change and one realistic unsupported extension, recording successful descriptor output, actionable refusal, and cleanup without requiring the downstream root command.

## Implementation Notes
