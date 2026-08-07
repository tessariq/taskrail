---
id: T-165-maintain-bounded-workflow-adversarial-review
title: Maintain bounded workflow-adversarial review memory
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-240-implement-the-normative-review-schema-decoders
    - T-250-render-prompts-from-storage-neutral-context
updated_at: "2026-08-04T21:32:13Z"
---

# T-165-maintain-bounded-workflow-adversarial-review Maintain bounded workflow-adversarial review memory

## Description

Add the sandboxed workflow-adversarial prompt/skill and strict report semantics so
stale evidence, dirty probes, repeated easy tests, and unevidenced closure cannot
masquerade as current assurance. Taskrail index derivation/publication is T-166.

## Acceptance

- Exact report objects retain never-reused IDs, terminal typed observation
  evidence, resolvable references, a role-mandated v1 prompt source/template
  binding, separate outcome/freshness, canonical ordering, 1 MiB input cap, and
  at most three explained surface keys per run.
- Canonical JSON and recorded-HEAD-tree hash framing produce byte-identical digests
  across platforms; obsolete closure uses superseding evidence while resolved and
  not-reproducible require fresh executed attempts.
- Review requires a clean source worktree, captures HEAD/spec/product snapshots
  before probing, excludes only the review subtree from product digest, and after
  cleanup permits only memory/report proposals; any other source diff forbids a
  clean claim.
- Source/spec/product changes stale rows by default unless a reviewed changed-path
  assertion retains freshness; Taskrail validates shape/digests, not semantic
  correctness. Review rotates surfaces and clean requires an observable probe.
- Finding transitions require human-created full task IDs for tracked, fresh
  reproduction for resolved, evidence/rationale for other closures, and
  visibility of unchecked findings.
- Review/probe/observation IDs, severity, nested ordering, evidence resolution,
  and closure evidence follow the exact schema; review IDs remain globally
  unique across version rollover.
- Version rollover retains applicable unresolved IDs/origin, marks prior surfaces
  stale, keeps report-local prompt bindings as historical evidence, does not copy
  them into `INDEX.json`, and keeps findings advisory.
- The packaged workflow-review skill retains Agent Skills-compliant frontmatter;
  installed copies use nested `metadata.taskrail_version`, with marker-free
  embedded and committed copies remaining byte-identical.

## Verification Notes

- Map criteria to schema/state fixtures, dirty/untracked source cases,
  prompt/source and digest/path changes, caps/rotation, sandbox cleanup, every
  disposition, output-only source diffs, and rollover.
- Produce a two-run report where clean becomes stale, a finding cannot close
  without reproduction, and a dirty source can never claim clean.
- Run Agent Skills conformance and package-parity checks for the workflow-review
  skill in addition to its behavioral fixtures.

## Implementation Notes
