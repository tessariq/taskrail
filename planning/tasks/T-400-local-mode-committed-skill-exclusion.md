---
id: T-400-local-mode-committed-skill-exclusion
title: Resolve local_exclusion_invalid on committed skill copies in local mode
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies: []
updated_at: "2026-09-11T16:28:44Z"
---

# T-400-local-mode-committed-skill-exclusion Resolve local_exclusion_invalid on committed skill copies in local mode

## Description

In the v0.5.0 skill evaluation (session v050-final-20260910t103851z), the
`taskrail-decompose-local` and `taskrail-repair-local` agents ran
`taskrail local status`, which reported `local_exclusion_invalid` for the
committed `.agents/skills/<skill>` copy with `promotion_ready: false`, while
`validate` passed. Adopters commonly commit packaged skills, so either local
mode's managed-exclusion set wrongly covers committed skill copies, or the eval
fixture is at fault.

Outcome: local status agrees with the intended contract for committed skill
copies in a local-mode repository. Reproduce before deciding which side changes.

## Acceptance

- A temp-repo reproduction (local init, committed skill copy, `local status`)
  confirms or refutes the report, and the decision is recorded here.
- If product: a committed skill copy outside managed local paths is not
  reported as `local_exclusion_invalid` and does not block promotion readiness.
- If fixture: the eval fixture stops tracking that path.

## Verification Notes

- Temp-dir test around `local status` for a committed skill copy.

## Implementation Notes
