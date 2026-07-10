# Skills Overview

Skill catalog for deterministic tracked-work execution in Taskrail.

## Canonical Skills

- `autonomous-backlog`
- `autonomous-manual-test`
- `autonomous-task`
- `autonomous-recovery`
- `autonomous-verify`

## Packaging

- Canonical dogfooding skill guidance lives in `skills/`.
- Mirrored runtime copies live in `.agents/skills/` and `.claude/skills/`.
- Productization decisions live in `docs/workflow/skills-productization.md`.
- `./scripts/check-skill-mirrors.sh` verifies the mirrored copies stay in sync.

## Shippable Skill Set

The repo-agnostic set installed by `taskrail init --with-skills` (T-030) lives
separately under `internal/taskrail/skills/`. It invokes the installed
`taskrail` binary (never `go run`) and creates tasks with `taskrail task new`
(the tracked-work skills) or `taskrail import --apply` (the onboarding skills).

Per `docs/workflow/skills-productization.md`, the split is:

- Shippable: `autonomous-backlog`, `autonomous-task`, `autonomous-verify`,
  `autonomous-recovery` (routes every correction through `taskrail repair`, never
  hand-editing state), and `taskrail-repair`, plus the product-only onboarding
  skills `taskrail-import` (notes/draft -> spec/task import) and
  `taskrail-retrofit` (guided retrofit of an existing repository into a Taskrail
  layout). The onboarding skills have no dogfooding counterpart under `skills/`;
  they exist only in the shippable set because Taskrail's own repository is
  already managed.
- Dogfooding-only: `autonomous-manual-test` (writes the internal
  `planning/artifacts/manual-test/` convention).

The dogfooding skills under `skills/` may keep `go run ./cmd/taskrail ...` until
the installed binary becomes the dogfooding entry point.

## Required Behavior

- current canonical (dogfooding) skills must route state transitions through
  `go run ./cmd/taskrail ...`; shipped skills must instead invoke the installed
  `taskrail` binary per `docs/workflow/skills-productization.md` Decision 1
- implementation skills must keep changes scoped to one selected task
- verification skills must point to concrete artifact paths
- all skills must preserve the same required flow and safety policy across mirrors
