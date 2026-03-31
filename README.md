# Taskrail

Taskrail is a deterministic execution harness for AI agents that turns goals into structured tasks, keeps work aligned to an authoritative state file, and advances execution through validation, verification, and follow-up.

## Status

This repository is bootstrapping Taskrail while also dogfooding the workflow style Taskrail is meant to standardize.

- Current target release: `v0.1.0`
- Active spec: `specs/v0.1.0.md`
- Planning state: `planning/STATE.md`

## What Taskrail Owns

- repo-native specs under `specs/`
- deterministic tracked work under `planning/`
- one authoritative `planning/STATE.md`
- task validation and dependency checks
- deterministic next-task selection
- explicit task transitions
- verification artifacts and follow-up tasks

## What Taskrail Does Not Own

- container sandboxing
- worktree lifecycle orchestration
- network policy enforcement
- distributed multi-agent runtime
- hosted control planes
- built-in LLM provider integrations in `v0.1.0`

## Repository Layout

```text
.
├── AGENTS.md
├── CHANGELOG.md
├── README.md
├── cmd/
├── docs/
├── internal/
├── planning/
├── scripts/
├── skills/
└── specs/
```

## Local Commands

- Build: `go build ./cmd/taskrail`
- Test: `go test ./...`
- Validate Taskrail structure: `go run ./cmd/taskrail validate`
- Select next task: `go run ./cmd/taskrail next`
- Show CLI help: `go run ./cmd/taskrail --help`
- Skill mirror check: `./scripts/check-skill-mirrors.sh`

## Read Next

1. `specs/v0.1.0.md`
2. `specs/v0.2.0.md`
3. `specs/v0.3.0.md`
4. `planning/STATE.md`
5. `AGENTS.md`

## Notes

- The versioned specs in `specs/` are normative.
- `planning/STATE.md` is the authoritative execution state for this repository.
- Temporary workflow scaffolding in `skills/`, `.agents/skills/`, `.claude/skills/`, and `docs/workflow/` exists to dogfood the Taskrail style before the product replaces it.
