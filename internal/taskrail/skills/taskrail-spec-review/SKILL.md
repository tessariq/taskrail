---
name: taskrail-spec-review
description: Stage and publish digest-bound independent post-spec review lenses without semantic writes
---

# taskrail-spec-review

Review a coherent specification before decomposition with four independent,
advisory lenses. The binary never calls a model; the agent produces untrusted
proposal files and a human alone decides dispositions. This skill never edits or
activates specs, never creates tasks, and never invokes semantic writers.

Requires the installed `taskrail` binary on `PATH`. Run it from the managed
repository root.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Flow

1. **Select an exact subject.** Run `${TASKRAIL:-taskrail} spec show <version> --json`
   and consume its `path`, exact content, and reported `sha256`; do not reopen or
   re-hash the logical or local-overlay path. Run `${TASKRAIL:-taskrail} status --json`
   and consume its exact `storage.artifacts_dir`. Choose one portable lowercase
   `<session-id>` and create its effectively ignored proposal directory
   `<proposal-dir>` beneath that reported transient directory at
   `<artifacts-dir>/review-proposals/spec/<session-id>`: rendering refuses a
   missing proposal directory, so it must already exist before the first
   `prompt render`, while the round files inside it stay absent until each lens
   writes them. Do not start this flow while the specification is still
   incoherent or while an operator has not selected the version and session.
2. **Render and hand off four isolated lenses per round.** Rounds number
   consecutively from 1; round `<n>` stages each lens at
   `<proposal-dir>/round-<n>-<lens>.json`. For each lens, render its own prompt
   with the same selected version, proposal output file, and session, for
   example round 1:
   `${TASKRAIL:-taskrail} prompt render spec-consistency --spec <version> --review <proposal-dir>/round-1-consistency.json --json`
   `${TASKRAIL:-taskrail} prompt render spec-gaps --spec <version> --review <proposal-dir>/round-1-gaps.json --json`
   `${TASKRAIL:-taskrail} prompt render spec-additions --spec <version> --review <proposal-dir>/round-1-additions.json --json`
   `${TASKRAIL:-taskrail} prompt render spec-adversarial --spec <version> --review <proposal-dir>/round-1-adversarial.json --json`
   Consume each JSON envelope. Give each reviewer the selected specification and
   relevant repository contracts, but no other lens observations: when supported,
   use separate contexts; otherwise record `same-context`, without earlier
   conclusions as facts. Each produces one schema-v1 JSON object at its
   round-scoped filename, including the exact prompt ID, v1 contract, template
   digest, source, session, that round's spec digest, and `fresh` or
   `same-context` identity. All four files of one round share one session ID and
   that round's exact spec path/digest. Finding IDs use the lens's disjoint
   namespace: `CONS-` for consistency, `GAPS-` for gaps, `ADDS-` for additions,
   and `ADV-` for adversarial. IDs remain unique across the complete session.
3. **Stop for human dispositions.** Findings remain advisory and open in their
   lens files; a lens observation never encodes the human's final decision. Ask
   the human for every decision and its rationale and must stop rather than
   manufacture them: never author dispositions, approval times, or identity
   claims. An `approved_at` timestamp, approval flag, or self-authored identity
   cannot authenticate a human decision, so the schema-2 manifest carries none of
   them and instead labels its `disposition_provenance` exactly
   `unverified-caller-recorded-claims`. For every retained finding occurrence,
   a human records exactly one disposition in `manifest.json`. Each disposition
   has exactly `round`, `finding_id`, `lens`, `severity`, `disposition`,
   `rationale`, and optional `resulting_spec_ref` or `target_version`; its lens
   and severity must equal the referenced finding occurrence in that round.
   `resulting_spec_ref` is required for accepted findings only and names a live
   resulting spec heading, and `target_version` is required for deferred findings
   only and names a future version. Rejected findings forbid both optional
   fields, and a high or medium finding cannot be deferred: publication and
   decomposition require every high/medium occurrence accepted or rejected.
   Additions never silently expand scope: explicitly decide whether they are
   current, future, or rejected.
4. **Batch edits before final observations.** A human may batch accepted spec
   edits outside this skill. Any changed spec byte stales all four lens
   observations, so rerun all four as the next round against the final digest
   and rebuild the manifest. A rerun after accepted spec edits retains earlier
   rounds and findings: a later clean round must never erase earlier findings or
   their recorded decisions and rationales, and every retained occurrence
   requires an explicit disposition. Unchanged exact bytes never justify another
   lens round; an unchanged-byte repeat that nonetheless occurs must be
   disclosed rather than discarded, by labeling that round
   `repeats_earlier_spec` in the manifest. Never drop a round, a finding, or a
   disposition over a manifest error; a refused publication requires a fresh
   valid proposal, not a trimmed history. A prompt-template drift stales the
   affected unpublished observation and requires a fresh lens response, not
   metadata repair. Do not begin decomposition until the final manifest is
   complete.
5. **Build the digest-bound history manifest.** `manifest.json` has exactly
   schema version 2, the shared session/spec path, the final round's spec
   digest, canonical generated UTC time, the fixed
   `unverified-caller-recorded-claims` provenance label, an ordered declared
   round history, and all dispositions. Each round entry has exactly `round`,
   `spec_sha256`, `repeats_earlier_spec`, and four ordered lens entries
   (consistency, gaps, additions, adversarial); each lens entry has exactly
   `lens`, `path`, `sha256`, `spec_sha256`: `path` is the fixed round-scoped
   filename (`round-<n>-<lens>.json`), `sha256` the exact SHA-256 of that lens
   file's bytes, and `spec_sha256` that round's spec SHA-256. The last declared
   round is final and its digest equals the manifest's final spec digest. The
   manifest does not repeat prompt bindings: lens file digests bind those
   transitively. Reject unknown, null, duplicate, missing, or malformed data
   rather than repairing it.
6. **Publish the complete bundle.** After rechecking the final spec digest,
   immediately before the non-dry-run publisher, apply the source-checkout guard.
   Then use the only publication boundary:
   `${TASKRAIL:-taskrail} review publish --type spec --proposal <proposal-dir> --destination <planning-dir>/reviews/spec/<version>/<session-id> --spec <version> --expect-spec-sha256 <digest> --json`
   It atomically publishes only the declared round files plus `manifest.json` to
   an absent destination. A publication refusal for a stale subject, prompt,
   digest, alias, hidden unchanged-byte repeat, or existing destination requires
   a fresh valid proposal; do not repair metadata or partially copy files.

## Rules

- lenses assess specification prose only; report sizing only for inseparable
  outcomes, contradictory boundaries, or missing integration ownership, never
  for proposed or existing task size
- do not use `coverage --gaps` as a substitute: it is a mechanical task-graph
  signal, while these are semantic pre-decomposition reviews
- do not let a lens edit specs, activate a version, create tasks, invoke import,
  change lifecycle state, or make final dispositions; never edit or activate specs
  and never create tasks
- do not decompose from an unpublished, incomplete, stale, or unresolved
  high/medium review bundle
- keep proposal files transient and use the generic `review publish --type spec`
  command for the sole durable write
