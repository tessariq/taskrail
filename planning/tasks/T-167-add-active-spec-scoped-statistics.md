---
id: T-167-add-active-spec-scoped-statistics
title: Add active-spec scoped statistics
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#active-spec-scoped-statistics
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
updated_at: "2026-08-26T08:06:08Z"
completion_id: "fbe52e202700c818aeba5bafe4344591"
last_verification_id: "e94a107fccb7f0546de72e970bb91566"
last_verification_result: pass
last_verified_at: "2026-08-26T08:06:08Z"
last_verified_completion_id: "fbe52e202700c818aeba5bafe4344591"
---

# T-167-add-active-spec-scoped-statistics Add active-spec scoped statistics

## Description

Add an opt-in active-spec projection to stats while preserving all existing
unfiltered output byte contracts, enforcing portable active-path coherence, and
retaining full-ledger dependency context.

## Acceptance

- The positive boolean flag works in text, JSON, DOT, and Mermaid modes; omission
  leaves existing output unchanged.
- Validate enforces slash-separated, case-correct canonical active path spelling,
  matching discoverable version/path, and no symlink traversal across supported
  filesystems.
- Subject, excluded, dependency-context, malformed-subject, and malformed-ledger
  sets follow exact definitions and ordering. One ordered issue list names every
  affected task/reference/classification and is the source of JSON/text/graph counts.
- Metrics use only subjects; traversal uses the full ledger and graphs mark
  off-spec context plus deterministic synthetic missing nodes with no dangling
  edge.
- Incoherence fails before output; zero cohorts succeed with N/A conventions; the
  command is read-only and the filter does not leak to other command families.

## Verification Notes

- Map criteria to default golden compatibility, exact scope outputs,
  mixed-spec/cancelled/missing/malformed/zero fixtures, graph oracles, and validate
  cases on case-sensitive/insensitive systems.
- Run every mode and invalid-path case while comparing full worktree bytes before
  and after.

## Implementation Notes

- 2026-08-26T08:05:45Z: Added active-spec scoped statistics with canonical active-spec validation, deterministic graph context, and regression coverage.
- 2026-08-26T08:06:08Z: verification pass id e94a107fccb7f0546de72e970bb91566 previous none completion fbe52e202700c818aeba5bafe4344591
