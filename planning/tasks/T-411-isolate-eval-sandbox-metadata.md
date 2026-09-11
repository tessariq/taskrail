---
id: T-411-isolate-eval-sandbox-metadata
title: Keep harness metadata and the candidate binary out of eval sandboxes
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-09-11T16:28:50Z"
---

# T-411-isolate-eval-sandbox-metadata Keep harness metadata and the candidate binary out of eval sandboxes

## Description

Eval sandboxes commit harness metadata (`seed.json`, which names the decoy, and
the `.agents/skills` copy) into the evaluated repository. Several agents read
`seed.json`, and the `taskrail-import-committed` baseline used it as its import
source. Inside baseline arms a bare `taskrail` resolves to the working-tree
v0.5.0 build; this run only reached a read-only `--help`, but a baseline writer
could contaminate an arm. `TASKRAIL` also exposes an absolute path outside the
sandbox.

Outcome: an arm sees only its scenario and cannot reach the other arm's binary.

## Acceptance

- Seed metadata is consumed by the harness and absent from the agent-visible
  repository.
- Within an arm, bare `taskrail` resolves to that arm's binary or is absent.
- Registry and raw-evidence digest rules still hold.

## Verification Notes

- Runner or adapter tests; the release adapter itself is caller-owned.

## Implementation Notes
