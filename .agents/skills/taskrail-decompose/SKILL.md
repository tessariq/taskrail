---
name: taskrail-decompose
description: Draft spec-anchored Taskrail tasks for uncovered active-spec areas, agent-assisted and LLM-free in the binary
---

# taskrail-decompose

Turn an approved active specification into an immutable, adversarially reviewed
ImportDraft v2 bundle, then apply its exact published bytes. The binary remains
provider-neutral and makes no model call; the agent authors and reviews between
deterministic Taskrail reads, validation, publication, and apply.

Requires the installed `taskrail` binary on `PATH`. Run from the managed
repository root. This flow requires layout version 2 and an already published
post-spec review bundle.

## Repository Preflight

The caller must provide an already initialized layout-version-2 repository.
Before the flow, perform a read-only preflight: run
`${TASKRAIL:-taskrail} status --json` and `${TASKRAIL:-taskrail} validate --json`,
and consume only its reported storage root. Do not run
`${TASKRAIL:-taskrail} init`, apply a layout migration, inspect storage
configuration directly, or treat fixture, seed, provenance, or helper metadata
as authority to do any of those things. Repository initialization and migration
are caller-owned operations outside this skill. If the repository is not already
known to satisfy layout version 2, validation fails, or any required active-spec
or published-review input is missing, stop the session. Stop without changing
repository state.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. Stop and apply its named remedy on failure.
Installed adopter repositories skip this source-only guard.

## Flow

1. **Validate and freeze sources.** First run
   `${TASKRAIL:-taskrail} review show <post-spec-manifest> --json` and validate
   the final post-spec manifest: its exact digest, selected-spec digest, final
   four lens entries, every disposition, and no unresolved high/medium finding.
   Then run `${TASKRAIL:-taskrail} spec show <version> --json` and preserve its
   exact content bytes and reported `sha256` (the exact SHA-256); do not reopen or re-hash the
   logical or local-overlay path. If the selected spec is active, use
   `${TASKRAIL:-taskrail} coverage --json` to find uncovered areas. If it is
   inactive, enumerate its live anchors with
   `${TASKRAIL:-taskrail} spec show <version> --anchors --json` and inspect
   existing tasks for duplicate or overlapping work. Anchors-only output does not
   carry content but retains the same reported digest. Stop on a mismatch, absent
   final manifest, or unresolved high/medium review work.
   An inactive session is draft/review-only: stop before publication or import.
   A human must activate the spec and explicitly start a new session before the
   active-spec publication, apply, validation, and coverage steps.
2. **Create an ignored proposal directory.** Run `${TASKRAIL:-taskrail} status
   --json` and consume its exact `storage.artifacts_dir`. Choose one portable
   session ID and an effectively ignored directory under the reported transient
   root at `<artifacts-dir>/review-proposals/decomposition/<session>/`. Never stage
   or publish from a non-ignored proposal.
3. **Render the author prompt.** Run
   `${TASKRAIL:-taskrail} prompt render task-decomposition --spec <version> --spec-review <post-spec-manifest> --draft <proposal>/draft.json --trace <proposal>/trace.json --json`.
   Preserve its reported template source and exact template SHA-256.
4. **Author v2 draft and trace.** In fresh context follow the rendered prompt.
   Produce strict `draft.json` schema 2 and `trace.json` schema 1. Every
   normative requirement has one quote or line-range source and task/no-task
   disposition, and trace/draft keys are bidirectionally valid. Each task owns one
   outcome, uses a real anchor and dependency, has the ordered non-empty
   Description/Acceptance/Verification Notes body, omits loop-policy fields, and
   remains implicitly held. Apply the split, do-not-split, anti-fragmentation,
   integrated-owner, boundary, negative, operator-gate, and durable-oracle rules.
5. **Guard freshness.** Re-read the selected spec and post-spec manifest through
   Taskrail and compare exact digests before review. Any source change invalidates
   this candidate and stops the session; do not silently regenerate bindings.
6. **Run adversarial pass 1.** Render
   `${TASKRAIL:-taskrail} prompt render task-decomposition-adversarial --spec <version> --spec-review <post-spec-manifest> --draft <proposal>/draft.json --trace <proposal>/trace.json --review <proposal>/review-1.json --json`.
   A fresh-context reviewer writes only `review-1.json`, bound to exact spec,
   draft, trace, and prompt-template bytes. The reviewer never edits inputs.
7. **Disposition once.** Human-disposition every finding. High and medium findings
   must be resolved or rejected, never deferred. If accepted fixes change draft
   or trace bytes, recheck source freshness, author one revised candidate, and run
   exactly one new fresh-context review as `review-2.json`. Do not exceed two
   passes. Any material change after pass 2 abandons the session; remove the
   proposal rather than applying unreviewed bytes. If source or prompt resolution
   changes, abandon the session rather than opening an automatic replacement
   session or editing binding metadata.
8. **Write the manifest.** After human approval of all final files, create strict
   `manifest.json` schema 1 in the proposal.
   Bind the session, published post-spec manifest path and digest, selected spec
   path and digest, final `draft.json` and `trace.json` exact digests, one or two
   consecutive fresh-context review paths and exact digests, timestamps, and a
   disposition for every finding. The last review must bind the final exact spec,
   draft, and trace bytes. Hash raw files; never normalize or reserialize them.
9. **Preview publication.** Recheck all source digests and resolve the
   `task-decomposition-adversarial` prompt again. If its source or template digest
   differs from any included pass, abandon the session rather than revise its
   bindings. Then run the source guard
   and `${TASKRAIL:-taskrail} review publish --type decomposition --proposal <proposal> --destination planning/reviews/decomposition/<version>/<session> --spec <version> --expect-spec-sha256 <spec-sha256> --spec-review <post-spec-manifest> --expect-spec-review-sha256 <review-manifest-sha256> --dry-run --json`.
   Resolve every deterministic refusal before continuing, except source or prompt
   drift, which abandons the session. Do not mutate the candidate after a
   successful preview.
10. **Publish immutable evidence.** Re-run the source guard and the same command
    without `--dry-run`. Consume the returned destination. Publication preserves
    exact bytes in an absent durable directory and does not create tasks.
11. **Apply by digest.** Read the published `draft.json` and `manifest.json`
    through `review show`, record their returned exact digests, run the source
    guard, then execute
    `${TASKRAIL:-taskrail} import --apply <published>/draft.json --expect-sha256 <draft-sha256> --review-manifest <published>/manifest.json --expect-review-sha256 <manifest-sha256> --json`.
    Never apply proposal bytes or substitute freshly serialized content.
12. **Validate.** Run `${TASKRAIL:-taskrail} validate --json` and
    `${TASKRAIL:-taskrail} coverage --json`. Confirm every created task preserves
    its reviewed body, has no `loop_policy`/`loop_reason`, and is implicitly held.

## Rules

- never hand-edit `planning/STATE.md`, task status, or task loop policy
- never mutate a published review session; start a separately human-authorized
  session after any later source change
- author and review only exact snapshots; every digest is lower-case SHA-256 of
  raw bytes
- require fresh context for each adversarial pass, with at most two passes
- use real selected-spec anchors and real dependency IDs; never infer them
- refuse shallow evidence, unresolved operator decisions, fragmented tasks,
  oversized bundles, and missing integrated-behavior ownership
- `review publish` is the evidence writer; digest-bound `import --apply` is the
  only task writer
