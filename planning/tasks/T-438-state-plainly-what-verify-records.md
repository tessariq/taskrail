---
id: T-438-state-plainly-what-verify-records
title: State plainly what verify records
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#verify-lifecycle-advisory
dependencies: []
updated_at: "2026-10-31T11:39:13Z"
---

# T-438-state-plainly-what-verify-records State plainly what verify records

## Description

Doc gap. README.md says verification "writes inspectable evidence" and gives
"reproducible local output" under "Make 'done' mean something"; specs/v0.2.0.md
says raw evidence is reproducible by re-running `verify`; docs/commands.md says
`verify` "records evidence". In fact `verify` runs nothing: it records the
caller's `--result` and `--summary` in gitignored artifacts, and committed state
keeps only the result, IDs and timestamp. Re-running `verify` creates a new
record rather than reproducing the old one. Readers can mistake a recorded
claim for an executed check. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- README.md and docs/commands.md describe `verify` as recording an attested result and summary, not running checks.
- The docs say which parts are committed and which stay local, and drop the 'reproducible' claim.
- The docs point to Verification Notes as the place where the checks a verifier must run are named.

## Verification Notes

- Read the updated README and docs/commands.md against `Service.Verify`.

## Implementation Notes
