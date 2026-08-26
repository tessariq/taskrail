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

## Flow

1. **Select an exact subject.** Run `${TASKRAIL:-taskrail} spec show <version> --json`
   and record its path and exact SHA-256. Choose one portable lowercase
   `<session-id>` and one absent proposal directory `<proposal-dir>` beneath
   `<artifacts-dir>/review-proposals/spec/<session-id>` for the active storage
   mode. Do not start this
   flow while the specification is still incoherent or while an operator has not
   selected the version and session.
2. **Render and hand off four isolated lenses.** For each lens, render its own
   prompt with the same selected version, proposal output file, and session:
   `${TASKRAIL:-taskrail} prompt render spec-consistency --spec <version> --review <proposal-dir>/consistency.json --json`
   `${TASKRAIL:-taskrail} prompt render spec-gaps --spec <version> --review <proposal-dir>/gaps.json --json`
   `${TASKRAIL:-taskrail} prompt render spec-additions --spec <version> --review <proposal-dir>/additions.json --json`
   `${TASKRAIL:-taskrail} prompt render spec-adversarial --spec <version> --review <proposal-dir>/adversarial.json --json`
   Consume each JSON envelope. Give each reviewer the selected specification and
   relevant repository contracts, but no other lens observations: when supported,
   use separate contexts; otherwise record `same-context`, without earlier
   conclusions as facts. Each
   produces one schema-v1 JSON object at its fixed filename, including the exact
   prompt ID, v1 contract, template digest, source, session, spec digest, and
   `fresh` or `same-context` identity.
3. **Inspect every observation with the human.** Findings remain advisory and
   open in their lens files. For every finding, a human records exactly one
   disposition in `manifest.json`: accepted, rejected, or deferred, with a
   non-empty rationale. Each disposition has exactly `finding_id`, `lens`, `severity`, `disposition`, `rationale`, and optional `resulting_spec_ref` or
   `target_version`; its lens and severity must equal the referenced finding.
   `resulting_spec_ref` is required for accepted findings only and names a live
   resulting spec heading, and `target_version` is required for deferred findings
   only and names a future version. Rejected findings forbid both optional fields.
   High and medium findings may be accepted, rejected, or deferred, but every
   finding requires its explicit disposition. Additions never silently expand
   scope: explicitly decide whether they are current, future, or rejected.
4. **Batch edits before final observations.** A human may batch accepted spec
   edits outside this skill. Any changed spec byte stales all four lens
   observations, so rerun all four against the final digest and rebuild the
   manifest. Unchanged exact bytes never justify another lens round. A
   prompt-template drift stales the affected unpublished observation and requires
   a fresh lens response, not metadata repair. Do not begin decomposition until
   the final manifest is complete.
5. **Build the digest-bound manifest.** `manifest.json` has exactly schema version
   1, the shared session/spec path/digest, canonical generated/approved UTC times,
   four ordered lens entries (consistency, gaps, additions, adversarial), and all
   dispositions. Each entry binds its fixed filename's exact SHA-256 and final
   spec SHA-256. The manifest does not repeat prompt bindings: lens file digests
   bind those transitively. Reject unknown, null, duplicate, missing, or malformed
   data rather than repairing it.
6. **Publish the complete bundle.** After rechecking the final spec digest, use
   the only publication boundary:
   `${TASKRAIL:-taskrail} review publish --type spec --proposal <proposal-dir> --destination <planning-dir>/reviews/spec/<version>/<session-id> --spec <version> --expect-spec-sha256 <digest> --json`
   It atomically publishes only the five fixed files to an absent destination.
   A publication refusal for a stale subject, prompt, digest, alias, or existing
   destination requires a fresh valid proposal; do not repair metadata or
   partially copy files.

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
