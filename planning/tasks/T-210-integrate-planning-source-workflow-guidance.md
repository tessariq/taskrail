---
id: T-210-integrate-planning-source-workflow-guidance
title: Integrate planning-source workflow guidance
status: todo
priority: medium
spec_ref: specs/v0.7.0.md#source-inspect-and-import-commands
dependencies:
    - T-209-wire-reviewed-planning-source-import
    - T-221-add-source-profile-and-receipt-inventories
updated_at: "2026-08-05T19:18:20Z"
---

# T-210-integrate-planning-source-workflow-guidance Integrate planning-source workflow guidance

## Description

Integrate the reviewed planning-source handoff into README, workflow,
provenance, upgrade/rollback, prompt, and packaged-skill guidance. The workflow
must be provider-neutral and make the trust boundary explicit: Taskrail hashes
and validates exact inputs while a human or external agent reviews meaning and
authors the final mapping and `ImportDraft` v3.

## Acceptance

- README and command/workflow documentation show the complete
  profile inventory -> inspect/existing receipt -> review -> draft/mapping ->
  preview -> apply -> receipt inventory flow, with
  exact command forms, preview as the default, `--apply` as the only write
  opt-in, local-spec anchor requirements, duplicate-snapshot behavior, and
  changed-snapshot append-only semantics.
- Guidance defines the reviewed handoff without preferring or requiring a model
  provider. It assigns semantic responsibility to the human/agent reviewer,
  treats final mapping and draft files as untrusted reviewed inputs, and states
  that digests, coverage, review assertions, and successful publication do not
  prove semantic completeness or authenticate the reviewer.
- The packaged import/decomposition/SDD-handoff/retrofit skills and relevant prompts guide agents to
  inspect the descriptor and exact source bytes, select real anchors in the
  chosen local Taskrail spec, produce exactly mapping v1 and `ImportDraft` v3,
  review every source and draft key, inspect preview against the existing
  ledger, and avoid embedding source paths or provenance in task bodies or
  frontmatter.
- Guidance makes retrofit establish the managed local spec first, then use source
  inspect/import; standalone retrofit remains non-skill-installing and ends with
  the explicit optional `init --with-skills` step.
- OpenSpec and Spec Kit guidance describes only the exact built-in v1 profile
  shapes and actionable `unknown_layout` refusal. It explicitly disclaims
  universal compatibility with upstream releases, forks, extensions, archives,
  templates, repository roots, renamed files, extra outputs, and deeper trees.
- Documentation consistently says interoperability handoff, not sync: there
  are no claims of model execution, semantic inference, source write-back,
  status/checkbox reconciliation, polling, watchers, task updates, conflict
  merging, deletion propagation, foreign lifecycle ownership, or receipt
  refresh/repair.
- Provenance and upgrade/rollback guidance explains canonical immutable
  receipts, historical rather than live bindings, duplicate tuple refusal,
  Git-based recovery for damaged receipts, v0.6 lifecycle compatibility and
  limitations, unchanged layout 3/state schema 2, and why new source imports
  require v0.7.
- Embedded skill sources and committed `.agents`/`.claude` copies remain
  byte-identical, provider-neutral, and free of executable source-system hooks,
  plugins, converters, network requirements, or hidden write behavior.

## Verification Notes

- Map workflow and command criteria to doc assertions over `README.md`, command
  help, and `docs/workflow/`, including exact preview/apply examples, local
  anchor examples, duplicate/changing snapshot outcomes, and rollback guidance.
- Map reviewed-handoff criteria to scenario tests or fixtures for the embedded
  import/decomposition skill and relevant prompts, proving mapping v1/v3 output,
  complete source/key review, preview review, and rejection of external
  `spec_ref` examples.
- Map profile-limit and no-sync criteria to focused terminology scans across
  README, docs, help, prompt, and skill sources that require narrow v1 limits
  and reject universal compatibility, synchronization, update, write-back, and
  provider-specific claims.
- Map provenance/compatibility criteria to documentation checks for canonical
  immutable historical receipts, Git recovery, layout/state/v3 stability, and
  the exact capabilities and limitations of a packaged v0.6 binary.
- Run `task skills:regen` and `task check:skills`; retain parity and workflow
  transcript evidence in `planning/artifacts/manual-test/T-210/<timestamp>/report.md`
  alongside links to the final README/docs/help/skill assertions.

## Implementation Notes
