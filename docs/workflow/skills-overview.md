# Skills Overview

Skill catalog for deterministic tracked-work execution in Taskrail.

## Canonical Skills

- `autonomous-backlog`
- `autonomous-manual-test`
- `autonomous-task`
- `autonomous-recovery`
- `autonomous-verify`

## Packaging

- Canonical skill guidance lives in `skills/`.
- Mirrored runtime copies live in `.agents/skills/` and `.claude/skills/`.
- `./scripts/check-skill-mirrors.sh` verifies the mirrored copies stay in sync.

## Required Behavior

- skills must route state transitions through `go run ./cmd/taskrail ...`
- implementation skills must keep changes scoped to one selected task
- verification skills must point to concrete artifact paths
- all skills must preserve the same required flow and safety policy across mirrors
