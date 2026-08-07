---
id: T-254-make-every-packaged-skill-storage-neutral
title: Make every packaged skill storage-neutral
status: todo
priority: high
spec_ref: specs/v0.5.0.md#storage-neutral-packaged-skills
dependencies:
    - T-165-maintain-bounded-workflow-adversarial-review
    - T-202-ship-the-lightweight-sdd-handoff-skill
    - T-213-define-the-uniform-agent-machine-api
    - T-216-ship-digest-bound-existing-task-review
    - T-223-run-every-v0-5-command-against-local-storage
    - T-235-show-a-task-by-exact-id-through-active-storage
    - T-242-align-full-task-skills-with-the-canonical
    - T-247-install-packaged-skills-safely-in-local-mode
updated_at: "2026-08-07T19:55:47Z"
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
  the path for that purpose, and durable writes return through sanctioned Taskrail
  commands.
- A4. T-242's full-task fixtures and every other skill-producing task compose into
  one package-level matrix with no conflicting storage, lifecycle, or delivery
  instruction; T-254 does not re-own their feature implementation.
- A5. Planning, review, recovery, verification, import, retrofit, spec, gap, and
  decomposition skills neither stage local metadata nor invoke `local promote`
  without an explicit human request.
- A6. A checked per-skill inventory classifies each observable as
  storage-independent or explicitly mode-specific before execution. Installed
  local copies are discoverable from both supported assistant roots;
  storage-independent observables match exactly, while every mode-specific
  difference matches its enumerated path, artifact, or Git-delivery oracle.
- A7. Embedded and committed copies remain provider-neutral, Agent
  Skills-compliant, marker-free, byte-identical, source-checkout-freshness-aware,
  and covered by the derived skill registry.

## Verification Notes

- A1-A3: static command/path mutation checks plus committed/local sandboxes with
  stale decoy logical files prove machine consumption and subject-command reads.
- A4-A6: the checked applicability inventory drives executable skill fixtures over
  exact index, worktree, commit, ignored state, assistant discovery, transient
  artifacts, and promotion behavior.
- A7: run frontmatter validation, provider scans, skill regeneration/parity,
  freshness checks, and the derived registry suite.

## Implementation Notes
