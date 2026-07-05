# Changelog

All notable user-visible changes to Taskrail will be documented in this file.

## Unreleased

### Added

- `taskrail task new` scaffolds a task file with the next free id and a template body (Description, Acceptance, Verification Notes, Implementation Notes). Requires `--title` and `--spec-ref`; supports `--priority`, repeatable `--dep`, and `--json`. It mirrors `validate`'s checks at creation time (spec anchor must resolve, dependencies must exist, priority must be valid) so an invalid task never lands, and increments the committed `STATE.md` todo count via the existing state-count logic.
- `taskrail import <source> --to tasks|spec|planning` deterministically parses a markdown source into T-032 draft form with no LLM calls: headings become spec sections, and subheadings plus list items become task drafts. It previews by default and writes reviewable draft files under `planning/imports/` only with `--apply` (overridable via `--out`, constrained to the repository), never modifying the source. The `planning` target additionally emits a non-authoritative STATE seed. The written draft is a valid `ImportDraft` and doubles as the `--apply` ingest target for the agent-driven path (T-034). Supports `--json`.
- Path discovery now reads an optional `.taskrail/config.yml` layout marker (`layout_version` plus `specs_dir`/`planning_dir` locations). When the marker is absent, discovery falls back to the v0.1.0 layout unchanged, so existing repositories need no migration.
- `taskrail init` is now version-aware and non-destructive. An empty repository gets the full layout plus a `.taskrail/config.yml` marker at the current `layout_version`; an existing unmarked v0.1.0 layout is adopted by writing only the marker (no other files change); and a marker recording an older `layout_version` triggers a migration that defaults to a dry run printing the diff, applies only with `--apply`, and re-runs validation. Migration never deletes or rewrites human-authored content under `specs/` or `planning/`. Supports `--json`.
- Homebrew install support via the `tessariq/homebrew-tap` tap: `brew install tessariq/tap/taskrail` (macOS and Linux). The v0.1.0 formula is published retroactively.

### Changed

- `taskrail verify` now records a portable result in committed `STATE.md`: `last_verification_result` is a path-free summary (result, task id, timestamp) and `relevant_artifacts` no longer lists gitignored `planning/artifacts/...` paths. Local evidence is still written under `planning/artifacts/verify/` for the producer, so a teammate cloning the repo no longer sees `STATE.md` pointing at files that exist only on the producer's machine.

### Fixed

- `taskrail validate` no longer requires the gitignored `planning/artifacts` and `planning/artifacts/verify` directories to exist. Git cannot track these empty output directories, so a freshly init-ed and committed repository previously failed `validate` on a clean checkout (fresh clone or CI). A missing artifacts tree is now treated as "no artifacts yet"; `taskrail verify` still creates the directories on demand.

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
