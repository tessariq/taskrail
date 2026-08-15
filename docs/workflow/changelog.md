# Changelog Policy

How `CHANGELOG.md` entries are written in this repository. The checklist in
`AGENTS.md` and the PR list in `CONTRIBUTING.md` link here instead of restating
the policy.

## When to add an entry

- Add an entry under `## Unreleased` for **user-visible behavior changes** only.
- Skip internal-only refactors, CI plumbing, and dependency-bump noise.
- Fold one user-facing feature into **one entry** even when it spans several
  tasks.

## How to write it

- Keep entries terse: **one to two lines**.
- Lead with the command or user-facing verb; state the observable effect and the
  flags a user types.
- Leave out internal mechanics (function names, struct/schema ids, `embed.FS`,
  "shared validator") and design rationale — those belong in the commit body or
  spec.
- Copy-edit against the existing entries so register and length stay consistent;
  the terse v0.1.0 entries are the reference.

## Examples

- Good: `` `taskrail repair` — reconcile mechanical `STATE.md` drift; dry run by
  default, `--apply` writes `STATE.md` only and re-validates. Supports `--json`. ``
- Bad: a 5-sentence paragraph restating the task description and how it works
  internally.
