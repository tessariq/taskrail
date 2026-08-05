---
id: T-209-wire-reviewed-planning-source-import
title: Wire reviewed planning-source import
status: todo
priority: high
spec_ref: specs/v0.7.0.md#source-inspect-and-import-commands
dependencies:
    - T-205-add-the-built-in-openspec-planning-profile
    - T-206-add-the-built-in-spec-kit-planning-profile
    - T-208-publish-strict-planning-provenance-sidecars
updated_at: "2026-08-05T19:18:16Z"
---

# T-209-wire-reviewed-planning-source-import Wire reviewed planning-source import

## Description

Wire `taskrail source inspect` and `taskrail source import` into the CLI as the
reviewed planning-source handoff. Inspection and default import preview are
read-only; `--apply` alone publishes fresh tasks, projected `STATE.md`, and one
canonical receipt as a single recoverable outcome after all inputs and
destinations are rechecked under the repository mutation lock.

## Acceptance

- `source inspect --profile <openspec|spec-kit> --root <repo-path> [--json]`
  recognizes exactly the built-in profile/version, validates one complete clean
  snapshot, and emits the deterministic ordered descriptor. It creates no lock,
  temporary repository file, normalized source, task, state, receipt, or
  provenance directory.
- `source import` requires profile, root, spec version, v3 draft, and v1 mapping,
  previews by default, and performs every non-publication parse, digest,
  duplicate, anchor, mapping, ledger, identity, dependency, allocation,
  candidate-task, state, receipt, and post-validation check. Preview reports
  `applied:false` and exact candidate task and receipt paths without reserving
  IDs or authorizing a later apply.
- `--apply` is the sole write opt-in. Under the common mutation lock it rechecks
  the complete source/input/receipt/ledger/state read set, performs final
  allocation, and atomically publishes exactly new live task files,
  `planning/STATE.md`, one receipt, and newly required provenance directories.
  Source files, local spec, draft, mapping, existing or archived tasks, prompts,
  skills, configuration, artifacts, and layout metadata are never written.
- Candidate validation runs before first publication and after publication.
  Failure injection at every task/state/directory/receipt, backup, and fsync
  boundary proves ordinary failure leaves no candidate output, while
  compare-and-swap rollback ambiguity retains recovery evidence and reports
  `partial_write`, `rollback_failed`, or `recovery_pending` without overwriting
  external edits.
- Apply refuses active or retained recovery, changed inputs, mixed snapshots,
  destination collisions, duplicate receipts, stale anchors, or ledger races.
  Retry after complete rollback repeats duplicate detection and allocation;
  tasks, state, and receipt can never be reported successful independently.
- Text and JSON modes have identical ordering, warnings, exit classification,
  and semantics. JSON uses the exact schema-version-1 command envelopes and
  inspect/import result fields; errors use only the specified stable codes,
  sorted paths, ordered violations, non-null arrays, and one uncontaminated
  stdout document with no unknown fields.
- Help and argument validation expose exactly the specified command forms and
  reject unknown profiles, invalid path/input combinations, unsupported draft
  versions, and incompatible flags with the correct stable error class. Neither
  command executes network access, hooks, source-system tools, scripts,
  templates, plugins, generated commands, commits, or pushes.

## Verification Notes

- Map command registration, flags, help, argument errors, exact text output, and
  machine envelopes to CLI smoke tests and text/JSON golden files under
  `cmd/taskrail/` and `internal/taskrail/`.
- Map preview purity to clean-tree and filesystem-manifest tests for successful
  and failing inspect/import runs; assert no lock, temp, provenance directory,
  task, state, source, spec, draft, or mapping byte changes.
- Map apply publication to integration tests that compare predicted and applied
  task/receipt output, then vary the ledger before apply to prove full recheck
  and valid reallocation rather than preview reservation.
- Map atomicity and recovery to deterministic failure-injection tests before
  and after each task, state, created-directory, receipt, backup, fsync, and
  post-validation boundary, including rollback races and retained-recovery
  refusal with complete digest diagnostics.
- Map output/error criteria to unknown-field schema tests and golden matrices
  for every stable top-level code, sorted paths, validation-phase violation
  order, empty non-null arrays, JSON stdout purity, and text/JSON exit parity.
- Record OpenSpec and Spec Kit inspect/preview/apply transcripts, manifests,
  rollback diagnostics, and no-input-write checks in
  `planning/artifacts/manual-test/T-209/<timestamp>/report.md` using the built
  working-tree binary in disposable Git sandboxes.

## Implementation Notes
