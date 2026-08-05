---
id: T-214-bootstrap-and-migrate-human-owned-repository-notes
title: Bootstrap and migrate human-owned repository notes
status: todo
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-05T20:24:12Z"
---

# T-214-bootstrap-and-migrate-human-owned-repository-notes Bootstrap and migrate human-owned repository notes

## Description

Add a human-owned repository notes sidecar without reviving generated STATE
continuation prose. Bootstrap it conservatively and provide the validated
no-clobber extraction candidate consumed by layout-upgrade tasks.

## Acceptance

- Applied fresh init and retrofit create a short commented `<planning-dir>/NOTES.md` template only when absent; preview and common JSON report it without writing.
- No-clobber note candidates resolve through the supplied planning context rather
  than hard-coded root paths; T-222 reuses the completed helper for local storage
  and T-224 owns later promotion.
- Existing regular notes remain byte-identical. Symlink/reparse, alias, and non-regular destinations fail closed; default and `--with-skills` modes behave identically.
- Provide strict extraction candidate/validation helpers that preserve decoded text/order under a labeled section and refuse an existing notes file; T-157 integrates those helpers into layout-2 preview/apply and its state/layout transaction.
- Agents are instructed to read the sidecar when relevant and edit it only on explicit human request; this task does not remove or rewrite schema-1 STATE notes.

## Verification Notes

- Exercise init/retrofit preview/apply, custom planning directories, absent/existing/unsafe notes paths, and exact before/after byte manifests.
- Unit-test empty/single/multiple/multiline legacy-note extraction candidates, exact text/order, existing-file refusal, aliases, and unsafe destinations. T-157/T-185 own transactional direct/multi-hop migration, rollback, schema removal, and old-binary refusal.

## Implementation Notes
