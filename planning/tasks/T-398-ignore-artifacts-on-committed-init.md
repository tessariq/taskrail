---
id: T-398-ignore-artifacts-on-committed-init
title: Ignore artifact output in committed-mode init
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-413-adopt-pre-staged-baseline-arms
    - T-414-publish-skill-eval-report
updated_at: "2026-09-11T16:21:42Z"
---

# T-398-ignore-artifacts-on-committed-init Ignore artifact output in committed-mode init

## Description

The artifact contract (v0.2.0 "Artifact And Init Consistency" and "Portable
Committed State") says generated output under `planning/artifacts/` is
deliberately gitignored, and packaged skills repeat it ("manual-test artifacts
… are ephemeral, gitignored evidence"). Committed-mode `taskrail init` never
makes that true: it writes no `.gitignore` entry and nothing to
`.git/info/exclude`, so every adopter repository shows verify, manual-test,
run, and review-proposal output as untracked files that one `git add -A`
would commit. This repository only satisfies the contract because its own
`.gitignore` lists those directories by hand. Local-mode init already proves
exclusion through `.git/info/exclude`.

Surfaced by the v0.5.0 paired skill evaluation (T-174): in
`autonomous-manual-test-committed` the candidate agent left its evidence
untracked, and the v0.4.0 baseline agent invented a self-ignoring
`planning/.gitignore` to hide it — both reacting to the same missing rule.

Outcome: a freshly committed-mode-initialized repository keeps Taskrail
artifact output out of Git status without hand configuration. The mechanism
(committed `.gitignore` entry versus Git-resolved exclude, and how upgrade
treats existing repositories) touches the init contract, so settle it with the
maintainer before implementation.

## Acceptance

- After committed-mode `taskrail init` in a fresh Git repository and a
  `taskrail verify`, `git status --porcelain --untracked-files=all` lists no
  path under the configured artifacts directory.
- Re-running `init` is idempotent: no duplicate ignore entries and no change
  to unrelated user-authored ignore content.
- A freshly initialized, committed repository still passes
  `taskrail validate` on a clean clone (the v0.2.0 invariant).

## Verification Notes

- Reproduce the gap first: temp Git repo, `taskrail init --json`, create a
  file under `planning/artifacts/verify/`, observe it in `git status`.
- Automated coverage: temp-dir init test asserting the ignore outcome and
  idempotence; manual sandbox run of init → verify → git status.

## Implementation Notes
