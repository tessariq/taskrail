---
id: T-140-v0-4-0-gap-drift-pre-release-spec-check
title: v0.4.0 gap/drift - pre-release spec check
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#goals
dependencies:
    - T-095
    - T-096
    - T-097
    - T-098
    - T-099
    - T-100
    - T-101
    - T-102
    - T-103
    - T-104
    - T-105
    - T-106
    - T-107
    - T-108-next-filter-idle-selection-to-active-spec-tasks
    - T-109-slug-transliteration-warn
    - T-110-next-include-off-spec
    - T-111-rename-not-retitle
    - T-112-slug-length-cap
    - T-113-spec-diff
    - T-114-task-repoint
    - T-115-slug-nfd-folding
    - T-116-rename-body-heading
    - T-117-duplicate-notes-heading
    - T-118-slug-nfc-normalize
    - T-119-heal-duplicate-notes-headings
    - T-120-stale-skill-warning
    - T-121-skill-version-marker
    - T-122-layout-version-guard
    - T-123-contributor-binary-resolution
    - T-124-parity-copy-skew-exemption
    - T-125
    - T-126-rename-slug-length-cap
    - T-127-note-portability-guard
    - T-128-task-new-title-portability
    - T-129-preview-post-apply-validity-in-task-rename-dry-run
    - T-130-normalize-spec-ref-path-on-write
    - T-131-layout-marker-read-dedup
    - T-132-report-partially-created-tasks-when-import-apply
    - T-133-surface-the-partial-apply-result-in-import-apply
    - T-134-freshcheck-samefile-error
    - T-135-duplicate-heading-guard-all-sections
    - T-136-layout-marker-error-path-form
    - T-137-skills-backup-stat-error-path
    - T-138-mark-failed-spec-write-partial
    - T-139-heading-match-crlf-fences
updated_at: "2026-07-29T11:54:51Z"
---

# T-140-v0-4-0-gap-drift-pre-release-spec-check v0.4.0 gap/drift - pre-release spec check

## Description

Pre-release gap/drift analysis for the **v0.4.0** release, mirroring T-049 and
T-061. It runs after all implementation and follow-up work lands and gates the
release: v0.4.0 is not tagged until this check passes. The spec is the source of
truth (`specs/v0.4.0.md`); do not duplicate its commitments here.

## Release Check

- Re-read the spec's **Summary**, **Goals**, and every **Potential Features**
  area, plus **Caution**, **Recommendation About LLM Support**, and **Explicitly
  Excluded** as drift guards.
- Walk the implementation, tests, user documentation, packaged skills, and
  release notes against each intended goal and feature. A **gap** is a spec
  commitment not met; a **drift** is behavior that diverges from the spec's
  intent, silently outgrows it, or crosses an explicit boundary.
- Run `taskrail coverage --min 100` as the decomposition gate and confirm the
  report's implementation coverage is also 100%. Run `taskrail coverage --gaps
  --json`, classify every structural signal as a real gap or an explained false
  positive, then use the `taskrail-gap` skill for the semantic review the binary
  deliberately cannot perform.
- Confirm the gitignored-artifacts contract has no tracked placeholder under
  `planning/artifacts/`: absent output directories are valid and verification
  still creates its evidence tree on demand.
- Run the repository's release checks as evidence: `gofmt -l .`, `go vet ./...`,
  `go test ./...`, `go test -race ./...`, `task build:cross`,
  `task check:skills`, `task check:task-bodies`, `task taskrail:check`, and
  `taskrail validate`.
- Review the latest main-branch CI, Planning checks, and CodeQL results. Build a
  release-version binary with `VERSION=v0.4.0 task release` and confirm
  `./taskrail version` reports `v0.4.0`; run a GoReleaser snapshot before tagging.
- Finalize release metadata only after the gate is otherwise green: move the
  candidate notes from `## Unreleased` into a dated `## v0.4.0` section, update
  README release status/scope, and run the changelog version and notes guards.

## Acceptance

- Every v0.4.0 goal and feature is classified met, gap, or drift against the
  implementation and documentation; every exclusion is confirmed absent.
- Decomposition and implementation coverage are both 100%; every structural gap
  signal has a recorded disposition and the semantic gap review is complete.
- Every real gap or drift becomes a follow-up task before the final transition;
  `--result pass` is used only when no unresolved release blocker remains.
- The artifact tree contains no tracked placeholders, while on-demand verify
  artifact creation and clean-checkout validation remain intact.
- The full validation, test, cross-build, parity, task-body, freshness, release
  build, changelog-guard, and snapshot checks pass with evidence recorded.
- `CHANGELOG.md` has a dated, non-empty `## v0.4.0` section and README names
  v0.4.0 as current only when this gate is ready to pass.
- Closed with `taskrail verify T-140-v0-4-0-gap-drift-pre-release-spec-check
  --result pass|fail`; tagging remains a maintainer action after the release-prep
  commit is reviewed and main is green.

## Verification Notes

- Use the `autonomous-verify` and `taskrail-gap` skills for the semantic walk and
  follow-up creation. `taskrail verify` writes local evidence under
  `planning/artifacts/verify/T-140-v0-4-0-gap-drift-pre-release-spec-check/`.
- Record the manual release-build and snapshot checks under the gitignored
  `planning/artifacts/manual-test/T-140-v0-4-0-gap-drift-pre-release-spec-check/`
  tree; committed verification summaries must remain path-free.

## Implementation Notes

- This gate depends on every task from T-095 through T-139 so it cannot become
  eligible before all v0.4.0 implementation and follow-up work is completed.
- Cancelling a dependency requires removing it from this list in the same change;
  cancelled dependencies do not resolve and would intentionally keep the release
  gate blocked.
- Candidate changelog and README copy may be prepared before this task runs, but
  the dated release heading and "current release" claim are finalization steps of
  this gate.
