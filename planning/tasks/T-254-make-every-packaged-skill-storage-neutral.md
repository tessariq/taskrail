---
id: T-254-make-every-packaged-skill-storage-neutral
title: Make every packaged skill storage-neutral
status: completed
priority: high
spec_ref: specs/v0.5.0.md#storage-neutral-packaged-skills
dependencies:
    - T-162-productize-digest-bound-post-spec-review-lenses
    - T-306-ship-the-sandboxed-workflow-adversarial-review
    - T-202-ship-the-lightweight-sdd-handoff-skill
    - T-273-complete-machine-api-consumer-compatibility
    - T-216-ship-digest-bound-existing-task-review
    - T-291-prove-inherited-command-parity-in-local-storage
    - T-235-show-a-task-by-exact-id-through-active-storage
    - T-294-show-durable-review-artifacts-through-active
    - T-242-align-full-task-skills-with-the-canonical
    - T-302-refresh-local-packaged-skills-safely
    - T-303-align-native-task-producers-with-the-body-contract
    - T-304-align-imported-and-decomposed-task-bodies
updated_at: "2026-08-27T09:47:51Z"
completion_id: "06b5d3a6764f72808b0b7274f06c8291"
last_verification_id: "7d240c15b825eaf7ade86e5ba9dddeba"
last_verification_result: pass
last_verified_at: "2026-08-27T09:47:51Z"
last_verified_completion_id: "06b5d3a6764f72808b0b7274f06c8291"
---

# T-254-make-every-packaged-skill-storage-neutral Make every packaged skill storage-neutral

## Description

Integrate every v0.5 skill-producing outcome and retrofit the remaining inherited
packaged skills to consume Taskrail's active storage context through subject
commands and machine results. Feature tasks, including T-242 for full-task flows,
retain implementation ownership; this task owns the complete-package matrix and
the residual skills not otherwise changed. Skills must remain discoverable in
both install modes without opening logical managed paths, reconstructing the local
overlay, or applying committed-mode Git delivery rules to ignored local state.

## Acceptance

- A1. Every packaged skill invokes `${TASKRAIL:-taskrail}` without a storage-mode
  flag and uses schema-1 JSON whenever it consumes IDs, paths, warnings,
  eligibility, previews, lifecycle outcomes, storage mode, or failures.
- A2. Managed task, spec, and durable review bytes are obtained through
  `task show`, `spec show`, and `review show`; decoy logical files prove no skill opens
  those paths directly or derives `.taskrail/local/` semantic paths.
- A3. Physical transient output is used only when an exact Taskrail result reports
  the path for that purpose, including status `storage.artifacts_dir` in both
  modes, and durable writes return through sanctioned Taskrail commands.
- A4. Review-producing skills copy exact prompt ID/version/source/template-digest
  metadata from `prompt render --json` into leaf proposals; they never infer a
  replacement path, template hash, or physical managed location.
- A5. T-242's full-task fixtures and every other skill-producing task compose into
  one package-level matrix with no conflicting storage, lifecycle, or delivery
  instruction; T-254 does not re-own their feature implementation.
- A6. Planning, review, recovery, verification, import, retrofit, spec, gap, and
  decomposition skills neither stage local metadata nor invoke `local promote`
  without an explicit human request. Local delivery instructions follow
  repository-visible Git conventions, preserve identity/configuration, and exclude
  incidental ignored Taskrail provenance from commit metadata and unrelated
  product text. Frozen repository-visible policy governs generic Git conventions,
  while only caller-owned instruction outside managed planning authorizes exposing
  a local Taskrail identity/path in commit metadata; outcome-required
  product bytes do not by themselves authorize them.
- A7. A checked per-skill inventory classifies each observable as
  storage-independent or explicitly mode-specific before execution. Installed
  local copies are discoverable from both supported assistant roots;
  storage-independent observables match exactly, while every mode-specific
  difference matches its enumerated path, artifact, or Git-delivery oracle. The
  inventory distinguishes deterministic certification of shipped instructions/scripted
  fixtures from T-218's stochastic real-agent observations; neither claims that
  Taskrail inspects opaque agent tool use.
- A8. Embedded and committed copies remain provider-neutral, Agent
  Skills-compliant, marker-free, byte-identical, source-checkout-freshness-aware,
  and covered by the derived skill registry.

## Verification Notes

- A1-A4: static command/path/binding mutation checks plus committed custom-
  planning-directory and fixed-overlay local sandboxes with stale decoy logical
  files prove machine consumption, subject-command reads, exact transient paths,
  Git ignore/index preconditions, and no durable path leakage.
- A5-A7: the checked applicability inventory drives executable skill fixtures over
  exact index, worktree, full commit provenance, Git identity/configuration,
  ignored state, assistant discovery, transient artifacts, explicit policy/outcome
  exceptions, and promotion behavior.
- A8: run frontmatter validation, provider scans, skill regeneration/parity,
  freshness checks, and the derived registry suite.

## Implementation Notes

- 2026-08-27T09:47:30Z: Certified storage-neutral packaged skill instructions, inventory coverage, and regenerated mirrors.
- 2026-08-27T09:47:51Z: verification pass id 7d240c15b825eaf7ade86e5ba9dddeba previous none completion 06b5d3a6764f72808b0b7274f06c8291
