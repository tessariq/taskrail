---
id: T-186-resolve-layout-3-identity-collisions-with-a
title: Resolve layout-3 identity collisions with a reviewed map
status: todo
priority: high
spec_ref: specs/v0.6.0.md#layout-3-migration-and-compatibility
dependencies:
    - T-185-upgrade-repositories-transactionally-to-layout-3
    - T-179-resolve-stable-task-references-across-every
    - T-181-detect-durable-physical-task-path-references
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-182-define-exact-v0-6-machine-result-schemas
updated_at: "2026-08-04T23:06:23Z"
---

# T-186-resolve-layout-3-identity-collisions-with-a Resolve layout-3 identity collisions with a reviewed map

## Description

Add the collision-only identity-map extension so otherwise valid layout-2
repositories upgrade without unsafe manual identity edits.

## Acceptance

- Apply acquires migration ownership before source relationship/collision
  resolution and retains it through rollback/recovery; preview remains
  read-only.
- Strict input is exactly schema_version 1 plus non-null unique exact from/to
  mappings; unknown/null/duplicate data fails and preview is default.
- Only collision participants map to valid opaque IDs; enough map for uniqueness
  and exactly one positive claimant remains.
- Old exact-full-ID semantics precede frontmatter/filename/H1/known-machine
  rewrites; prose/history never rewrites, and old paths/post-map failures refuse.
- Human/JSON preview/apply reports every old/new ID/path and rewritten machine
  field; map/marker/skills/tasks/state/policy publish recoverably and no
  alias/key reuse remains.

## Verification Notes

- Map criteria to lock races, decoder/preview, collision groups, source
  semantics, archived claimants, path blockers, complete output, partial maps,
  death, and post-layout refusal.
- Persist one collision-heavy report proving every rewrite and numeric
  retention.

## Implementation Notes
