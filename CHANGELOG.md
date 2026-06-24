# Changelog

All notable user-visible changes to Taskrail will be documented in this file.

## Unreleased

### Added

- Path discovery now reads an optional `.taskrail/config.yml` layout marker (`layout_version` plus `specs_dir`/`planning_dir` locations). When the marker is absent, discovery falls back to the v0.1.0 layout unchanged, so existing repositories need no migration.
- Homebrew install support via the `tessariq/homebrew-tap` tap: `brew install tessariq/tap/taskrail` (macOS and Linux). The v0.1.0 formula is published retroactively.

### Changed

- `taskrail verify` now records a portable result in committed `STATE.md`: `last_verification_result` is a path-free summary (result, task id, timestamp) and `relevant_artifacts` no longer lists gitignored `planning/artifacts/...` paths. Local evidence is still written under `planning/artifacts/verify/` for the producer, so a teammate cloning the repo no longer sees `STATE.md` pointing at files that exist only on the producer's machine.

## v0.1.0 - 2026-06-19

First shippable release. Taskrail is a manual-first, LLM-provider-agnostic CLI for
repo-native tracked work, proving the repository contract, deterministic task
progression, the authoritative `STATE.md`, and verification as a first-class concept.

### Added

- `taskrail init` — initialize Taskrail structure (`specs/`, `planning/`, starter `STATE.md`) in the current repository.
- `taskrail validate` — validate folder layout, task shape, dependency and spec references, and `STATE.md` consistency.
- `taskrail next` — deterministically select the next eligible task (supports `--json`).
- `taskrail start <task-id>` — mark a task active and update `STATE.md`.
- `taskrail complete <task-id>` — mark a task completed from an implementation perspective (supports `--note`).
- `taskrail block <task-id>` — mark a task blocked and record a `--reason`.
- `taskrail verify <task-id>` — record a verification outcome and write artifacts under `planning/artifacts/verify/`; can create a follow-up task via `--create-followup`.
- `taskrail version` — print the CLI version (also `--version`), injected at build time via `-ldflags`.
- Bootstrap repository structure, specs, planning workflow, and mirrored skills.
