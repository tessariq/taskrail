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

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. Stop and apply its named remedy on failure.
Installed adopter repositories skip this source-only guard.

## Flow

1. **Freeze sources.** Run `${TASKRAIL:-taskrail} coverage --json`,
   `${TASKRAIL:-taskrail} spec show <version> --json`,
   `${TASKRAIL:-taskrail} spec show <version> --anchors --json`, and
   `${TASKRAIL:-taskrail} review show <post-spec-manifest> --json`. Preserve the
   plain show result's exact content bytes and compute their exact SHA-256; anchors-only
   output does not carry content or a digest. Record the post-spec manifest's
   exact returned SHA-256 and require its `spec_sha256` to equal the selected
   content digest. Stop on a mismatch or unresolved high/medium review work.
2. **Create an ignored proposal directory.** Choose one portable session ID and
   an effectively ignored directory under
   `planning/artifacts/review-proposals/decomposition/<session>/`. Never stage or
   publish from a non-ignored proposal.
3. **Render the author prompt.** Run
   `${TASKRAIL:-taskrail} prompt render task-decomposition --spec <version> --spec-review <post-spec-manifest> --draft <proposal>/draft.json --trace <proposal>/trace.json --json`.
   Preserve its reported template source and exact template SHA-256.
4. **Author v2 draft and trace.** In fresh context follow the rendered prompt.
   Produce strict `draft.json` schema 2 and `trace.json` schema 1. Each task owns
   one outcome, uses a real anchor and dependency, has the ordered non-empty
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
   passes. A source change stops rather than opening an automatic replacement
   session.
8. **Write the manifest.** Create strict `manifest.json` schema 1 in the proposal.
   Bind the session, published post-spec manifest path and digest, selected spec
   path and digest, final `draft.json` and `trace.json` exact digests, one or two
   consecutive fresh-context review paths and exact digests, timestamps, and a
   disposition for every finding. The last review must bind the final exact spec,
   draft, and trace bytes. Hash raw files; never normalize or reserialize them.
9. **Preview publication.** Recheck all source digests, then run the source guard
   and `${TASKRAIL:-taskrail} review publish --type decomposition --proposal <proposal> --destination planning/reviews/decomposition/<version>/<session> --spec <version> --expect-spec-sha256 <spec-sha256> --spec-review <post-spec-manifest> --expect-spec-review-sha256 <review-manifest-sha256> --dry-run --json`.
   Resolve every deterministic refusal before continuing; do not mutate the
   candidate after a successful preview.
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
