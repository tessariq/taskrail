---
id: T-165-maintain-bounded-workflow-adversarial-review
title: Maintain bounded workflow-adversarial review memory
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-159-add-a-versioned-workflow-prompt-catalog
    - T-201-make-packaged-skills-agent-skills-compliant
updated_at: "2026-08-04T21:32:13Z"
---

# T-165-maintain-bounded-workflow-adversarial-review Maintain bounded workflow-adversarial review memory

## Description

Add strict bounded workflow-adversarial index/report contracts and the sandboxed
review prompt/skill so stale evidence, dirty-source probes, repeated easy tests,
and unevidenced closure cannot masquerade as current assurance.

## Acceptance

- Exact schemas retain never-reused IDs, separate outcome/freshness, preserve
  unresolved findings, enforce caps, and add at most three explained surface keys
  per run.
- Review requires a clean source worktree, captures HEAD/spec/product snapshots
  before probing, excludes only the review subtree from product digest, and after
  cleanup permits only memory/report proposals; any other source diff forbids a
  clean claim.
- Source/spec/product changes stale rows by default unless evidenced path analysis
  retains freshness; review rotates among untested, stale, and shallow surfaces in
  an isolated sandbox and clean requires an observable probe.
- Finding transitions require human-created task refs for tracked, fresh
  reproduction for resolved, evidence/rationale for other closures, and
  visibility of unchecked findings.
- Version rollover retains applicable unresolved IDs/origin, marks prior surfaces
  stale, and keeps findings advisory.
- The packaged workflow-review skill retains Agent Skills-compliant frontmatter;
  installed copies use nested `metadata.taskrail_version`, with marker-free
  embedded and committed copies remaining byte-identical.

## Verification Notes

- Map criteria to schema/state fixtures, dirty/untracked source cases,
  digest/path changes, caps/rotation, sandbox cleanup, every disposition,
  output-only source diffs, and rollover.
- Produce a two-run report where clean becomes stale, a finding cannot close
  without reproduction, and a dirty source can never claim clean.
- Run Agent Skills conformance and package-parity checks for the workflow-review
  skill in addition to its behavioral fixtures.

## Implementation Notes
