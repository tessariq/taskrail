---
id: T-201-make-packaged-skills-agent-skills-compliant
title: Make packaged skills Agent Skills compliant
status: completed
priority: high
spec_ref: specs/v0.5.0.md#agent-skills-format-and-version-metadata
dependencies: []
updated_at: "2026-08-08T15:20:32Z"
---

# T-201-make-packaged-skills-agent-skills-compliant Make packaged skills Agent Skills compliant

## Description

Make every skill Taskrail ships conform to the Agent Skills frontmatter contract without weakening installed-copy version-skew protection. Packaged and committed skill sources remain marker-free and byte-identical, while the common installed-output transformation writes the running Taskrail version only as `metadata.taskrail_version`. This task owns committed installation and the reusable marker behavior; T-247 owns local CLI installation and its repository fixture. Existing installations with the legacy top-level marker remain readable and can be normalized safely, so adoption of the standard format does not force users to discard valid skills or conceal conflicting version evidence.

## Acceptance

- A1. Packaged skill validation requires string `name` and `description`, permits only the Agent Skills optional top-level fields `license`, `compatibility`, `metadata`, and `allowed-tools`, and rejects non-standard top-level Taskrail fields. All shipped skills and both committed mirrors satisfy that validation.
- A2. Shipped skill frontmatter no longer contains `argument-hint`; any invocation arguments remain explained in the skill prose.
- A3. Committed `taskrail init --with-skills` and the shared installed-output transformation consumed by T-247 write copies with a non-empty YAML string at `metadata.taskrail_version`, write no top-level `taskrail_version`, and preserve other valid metadata entries. T-247 proves `init --local --with-skills` end to end.
- A4. Version-marker reads accept a nested marker or the legacy top-level string. A file containing both is valid only when the decoded string values match; conflicting values fail install or refresh without changing the destination.
- A5. A successful refresh normalizes legacy-only and matching-dual markers to nested-only form. Invalid marker types, empty values, and conflicting dual values are rejected rather than guessed or silently repaired.
- A6. Nested and legacy markers use one running-version skew policy and the same advisory and `--force` remedy. Marker location cannot change whether a version is considered current or skewed.
- A7. Marker-free installed copies retain parity-aware behavior: bytes identical to the embedded package are accepted, while divergent bytes remain unknown-version and require the existing explicit remedy.
- A8. Embedded package sources plus `.agents` and `.claude` mirrors remain marker-free and byte-for-byte equal. Installed-output stamping is separate from package parity, and refresh skew checks still protect destination changes.
- A9. Layout migration classifies a marker-free byte-identical destination as a
  parity mirror and preserves it marker-free rather than stamping installed output.

## Verification Notes

- A1-A2: Unit-level frontmatter table tests should cover every allowed top-level field, missing required fields, unknown/custom fields, and removal of `argument-hint`; the package parity check should inspect all shipped copies.
- A3-A5: Boundary integration tests exercise committed installation plus the common installed-output transformation for no marker, nested-only, legacy-only, matching dual, conflicting dual, empty marker, non-string marker, and unrelated metadata. Assert exact resulting frontmatter for successful cases and unchanged destination bytes for refusals; T-247 supplies the local repository integration matrix.
- A6-A7: CLI integration tests should run current-version, older/newer-version, marker-free-identical, and marker-free-divergent destinations with and without `--force`, proving equivalent outcomes for legacy and nested locations.
- A8: Run the packaged-skill regeneration/parity check and task-body hygiene check; retain command output as verification evidence. A manual installed-copy inspection may supplement the automated checks but is not the oracle for parity.
- A9: Migration fixtures distinguish mirrors from installed/divergent copies in
  Taskrail's source checkout and an adopter repository.

## Implementation Notes

- 2026-08-08T15:20:16Z: Shipped Agent Skills-compliant packaged frontmatter, nested version metadata with strict legacy normalization, parity-safe migration behavior, and comprehensive conformance/skew tests.
- 2026-08-08T15:20:32Z: verification pass
