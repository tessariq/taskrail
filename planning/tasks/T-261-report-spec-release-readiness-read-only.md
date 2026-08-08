---
id: T-261-report-spec-release-readiness-read-only
title: Report spec release readiness read-only
status: todo
priority: high
spec_ref: specs/v0.6.0.md#read-only-spec-release-check
dependencies:
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-189-bind-archive-eligibility-to-verification
    - T-190-validate-unified-ledger-semantics-and-stable-read
    - T-195-report-unified-ledger-storage-without-semantic
    - T-259-add-explicit-agent-mode-and-structured-help
updated_at: "2026-08-08T11:20:11Z"
---

# T-261-report-spec-release-readiness-read-only Report spec release readiness read-only

## Description

Add `spec release-check` as a stable read-only report of mechanical planning
readiness for one selected version, without recording release state or performing
release operations.

## Acceptance

- Report active-spec, decomposition, open-work, implementation, and current
  verification checks in exact order over one stable unified-ledger snapshot.
- Live/archived completion, cancellation coverage, dependency validity, uncovered
  areas, stale completion-generation bindings, and invalid/recovery-fenced state
  follow the specified readiness and error rules.
- Ready and not-ready reports use the exact schema-version-2 result; not-ready
  exits non-zero without becoming an error, changing validation, or writing any
  task/spec/state/review/Git byte.
- Help and documentation state clearly that tests, builds, review, Git delivery,
  tagging, and publication remain separate release requirements.

## Verification Notes

- Exercise each readiness check independently and in combination, including
  cancelled-only areas, archived history, read races, non-active versions, and
  byte-for-byte read-only assertions in committed and local storage.

## Implementation Notes
