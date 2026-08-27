---
name: taskrail-workflow-adversarial
description: Run one bounded sandboxed workflow review and publish only validated report and memory evidence
---

# taskrail-workflow-adversarial

Perform one serial post-implementation adversarial review against an explicitly
selected spec. The binary remains provider-neutral: the agent chooses and probes
the bounded surface, while Taskrail validates and publishes the immutable report
and derived memory. This skill is report-only and never grants remediation or
lifecycle authority.

Requires the installed `taskrail` binary on `PATH`. Run from an explicitly
initialized managed repository root. Review publication never performs implicit
local bootstrap.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Flow

1. **Resolve storage and freeze the subject.** Run
   `${TASKRAIL:-taskrail} status --json` and consume the reported storage mode and
   `artifacts_dir`. Run `${TASKRAIL:-taskrail} spec show <version> --json`; preserve
   its exact content and compute its lower-case SHA-256 without normalizing bytes.
   Require a clean attached source worktree: reject detached HEAD, a bare
   repository, staged, unstaged, or untracked changes, and an unresolved Git
   operation. Record the full `HEAD` object ID. These rules apply in both committed
   and local storage modes; managed reads always use logical paths.
2. **Read prior memory through Taskrail.** Let `<memory>` be the logical
   `planning/reviews/workflow-adversarial/INDEX.json`. Run
   `${TASKRAIL:-taskrail} review show <memory> --json`. Treat only exact
   `review_not_found` as first-run empty memory and record `absent`; every other
   error stops. On success preserve the returned exact content and SHA-256. Never
   create a placeholder index.
3. **Choose one bounded review.** Inspect the selected spec, completed tasks,
   implementation, tests, and prior memory. Enumerate tasks through
   `${TASKRAIL:-taskrail} task loop list --json`, select completed rows relevant
   to the chosen surface, and read each exact managed task with
   `${TASKRAIL:-taskrail} task show <task-id> --json`; never open a logical task
   path directly. Select at most three normalized
   surface keys, preferring untested, stale, or shallow rows and each row's
   `next_angle`. Use a fresh context when supported; otherwise record
   `same-context` without claiming independence. Challenge end-to-end intent,
   lifecycle transitions, trust boundaries, stale evidence, failure recovery,
   cleanup, and cases where tests pass while users fail. Do not imply broad
   coverage from this bounded review.
4. **Authorize one proposal.** Choose a globally unique portable `<review-id>` and
   an absent effectively ignored `<proposal>` beneath the reported
   `<artifacts_dir>/review-proposals/workflow-adversarial/<review-id>`. Its sole
   eventual file is `report.json`. Run
   `${TASKRAIL:-taskrail} prompt render workflow-adversarial --spec <version> --memory <memory> --review <proposal>/report.json --json`.
   Consume the rendered content, source, contract version, and template SHA-256.
   The absent canonical memory path is valid only for the exact first-run case.
5. **Capture the product snapshot.** Compute `product_sha256` from the recorded
   `HEAD`, never from worktree bytes. Recursively enumerate the full Git tree,
   excluding the complete logical `planning/reviews/workflow-adversarial/`
   subtree, in unsigned UTF-8 path order. Hash ASCII
   `taskrail-workflow-product-v1`, one NUL, then for each entry: slash-separated
   path bytes, NUL, six-digit octal mode, NUL, unsigned decimal content length,
   NUL, exact blob bytes, NUL. For a gitlink use its lower-case full object ID as
   content. Reject malformed paths, unsupported entries, abbreviated IDs, or any
   inability to reproduce the exact framing.
6. **Probe only an isolated sandbox.** Create a temporary clone or detached
   worktree outside the source repository, managed storage, and proposal tree at
   exactly the frozen commit. Run every potentially mutating probe only in this
   isolated sandbox. A probe records terminal observable evidence as a command
   with exit status or an exact durable file. A bounded manual observation may
   support context, findings, or an inconclusive outcome, but never a clean claim
   or finding closure. Inspection or an existing suite result alone cannot
   support a clean outcome. Record inconclusive rather than guessing when setup
   or observation is insufficient.
7. **Clean up and prove isolation.** Remove the isolated sandbox and verify it is
   gone. A cleanup failure, source-worktree change, HEAD change, unexpected source
   output, or disallowed proposal output stops publication and forbids a clean
   claim. Re-read the spec, memory, and rendered prompt through Taskrail and
   recompute the product digest. Any changed source or prompt snapshot stales the
   candidate; do not repair bindings or silently restart.
8. **Stage exactly one report.** Read the Exact V1 Derivation Reference below,
   then write exactly one strict transient `report.json` following that schema, its
   candidate-index derivation algorithm, and the rendered prompt. Bind the exact prompt fields,
   review ID, spec/head/product snapshots, context mode, timestamps, no more than
   three surfaces, probes, observations, findings, prior-memory digest or
   `absent`, and the exact canonical candidate-index digest. Keep every collection
   sorted and duplicate-free. Clean requires an executed probe with referenced
   terminal observable evidence. Handle stale rows and finding dispositions
   conservatively: retain freshness only with affected-path evidence; closure and
   `not-reproducible` require fresh attempts; deferred or obsolete findings require
   evidence and rationale; tracked findings require a resolving task ID. Never
   invent, recycle, renumber, or silently drop a finding. Independently construct
   the exact canonical candidate `INDEX.json` bytes described by the reference,
   hash those bytes into `index_sha256_after`, and discard the local candidate;
   never stage or publish it yourself.
9. **Preview the public boundary.** Recheck the clean source and every frozen
   digest, then run the applicable exact command:
   `${TASKRAIL:-taskrail} review publish --type workflow --review <proposal>/report.json --memory <memory> --destination <destination> --spec <version> --expect-spec-sha256 <digest> --expect-head <head> --expect-product-sha256 <digest> --expect-memory-absent --dry-run --json`
   or replace `--expect-memory-absent` with
   `--expect-memory-sha256 <digest>`. The destination is the absent logical
   `planning/reviews/workflow-adversarial/runs/<version>/<review-id>.json`.
   Consume the complete JSON result and resolve deterministic report errors only
   by correcting the unpublished report from the frozen evidence. Subject,
   prompt, memory, product, or destination drift abandons this review ID.
10. **Publish unchanged evidence.** After a successful preview, immediately before
    the non-dry-run publisher, apply the source-checkout guard. Run the identical
    `${TASKRAIL:-taskrail} review publish --type workflow --review <proposal>/report.json --memory <memory> --destination <destination> --spec <version> --expect-spec-sha256 <digest> --expect-head <head> --expect-product-sha256 <digest> --json`
    command with the same memory expectation and `--json`, but without
    `--dry-run`. Consume the returned report and index paths and digests. Re-read
    both with `review show` and verify the exact published bytes. Remove the
    transient proposal after consuming the result. A human reviews
    and commits or deliberately discards this allowed report/memory diff before
    another serial run.

## Authority And Safety

- The skill never writes `INDEX.json` or a final run file directly; only
  `review publish --type workflow` may derive and atomically publish them.
- The skill never edits product code, specs, tasks, lifecycle status, task-local
  loop policy, verification results, or Git history and never promotes a finding.
- The skill never stages, commits, merges, pushes, creates refs, changes Git
  configuration, or otherwise integrates source-repository Git state.
- Do not run probes in the source worktree. Do not treat sandbox cleanup failure,
  source drift, stale memory, or unexpected outputs as a clean review.
- Do not expose physical local-overlay paths in reports. Proposal paths are
  transient and never valid terminal file evidence.
- One invocation performs one bounded review and publication attempt. It does not
  retry with a replacement context, launch parallel reviewers, or begin another
  review after publication.
- Remove an abandoned proposal directory after recording the refusal. Never leave
  a partial `report.json` or helper output behind.

## Exact V1 Derivation Reference

The report object has exactly these members: `schema_version:1`, `prompt_id` set
to `workflow-adversarial`, `prompt_contract_version:v1`, lower-case
`prompt_template_sha256`, `prompt_source` (`builtin|replacement`), portable
globally unique `review_id`, logical `spec_path`, lower-case `spec_sha256`, full
lower-case `tested_head`, lower-case `product_sha256`, `context_mode`
(`fresh|same-context`), canonical UTC `generated_at`, `scope`, non-null `probes`,
non-null `observations`, non-null `findings`, `index_sha256_before` (digest or
`absent`), and lower-case `index_sha256_after`.

`scope` is exactly `summary`, non-null `surfaces`, and non-null
`freshness_assessments`. A surface is exactly `surface_key`, `angle`, `rationale`,
`outcome`, non-null `evidence_refs`, non-null `finding_ids`, and `next_angle`.
Outcome is `clean|finding|inconclusive`. A freshness assessment is exactly
`surface_key`, `decision` (`stale|retain`), non-null ordered `changed_paths`,
`evidence`, and `rationale`. Surface collections sort by `surface_key`.

A probe is exactly `probe_id`, non-null `surface_keys`, `action`, `executed`,
`outcome` (`pass|fail|inconclusive`), non-null `observation_ids`, and non-null
`evidence_refs`. An observation is exactly `observation_id`, `probe_id`,
`expected`, `observed`, `outcome`
(`supports-clean|supports-finding|inconclusive`), and non-null `evidence`.

Terminal evidence is exactly `kind`, `summary`, nullable `path`, nullable
`sha256`, nullable `command`, and nullable integer `exit_code`. Kind is
`command|file|manual`. File paths are canonical durable product or final-review
logical paths; proposal, artifact, runtime, physical overlay, and active review
output paths are forbidden. Command evidence requires non-empty `command` and an
`exit_code`, with `path` and `sha256` null. File evidence requires canonical
`path` and lower-case 64-hex `sha256`, with `command` and `exit_code` null. Manual
evidence requires all four nullable members to be null. Only command or file
evidence from this review's executed probe can support clean, resolved, or
not-reproducible. An evidence reference is exactly `review_id`, `probe_id`, and
`observation_id`. Every reference resolves; all reference and scalar-ID arrays
are duplicate-free and sorted by unsigned UTF-8 tuple or scalar.

A report finding is exactly `finding_id`, `severity`, non-null `evidence_refs`,
`impact`, `status`, `rationale`, and optional `task_id`. Severity is
`high|medium|low`; status is
`open|tracked|resolved|not-reproducible|deferred|obsolete`. `tracked` requires a
resolving task ID. New IDs are `WF-` plus a positive decimal number padded to at
least six digits and no lower than prior `next_finding_number`. Existing IDs and
stored task IDs are immutable.

The candidate index has exactly `schema_version`, `next_finding_number`, non-null
`surfaces`, and non-null `findings`, in that order. Each surface has exactly, in
order: `surface_key`, non-null `evidence_refs`, `outcome`, `freshness`,
`spec_path`, `spec_sha256`, `product_sha256`, `tested_head`, `checked_at`, non-null
`finding_ids`, and `next_angle`. Each finding has exactly, in order: `finding_id`,
`severity`, non-null `evidence_refs`, `impact`, `first_seen`, `last_checked`,
`status`, `rationale`, and optional `task_id`. Each snapshot is exactly
`review_id`, `spec_path`, `spec_sha256`, `product_sha256`, `tested_head`, and
`checked_at`. Index status is only `open|tracked|deferred`.

Derive the candidate as follows:

1. Strictly decode prior memory and require `index_sha256_before` to match exact
   bytes. On first run use semantic version-1 empty memory with counter 1 and
   empty arrays, while requiring `absent`.
2. Validate every report finding transition. New findings may only be open,
   tracked, or deferred, cannot use a number below the prior counter, and may add
   a task ID only when tracked. Never change a stored task ID.
3. Remove only named resolved, not-reproducible, or obsolete findings. Replace
   named unresolved findings; preserve unnamed prior findings; sort numerically.
4. New unresolved findings use the current review/spec/product/HEAD/time snapshot
   for both `first_seen` and `last_checked`. Existing findings preserve
   `first_seen`; they preserve `last_checked` unless this report has a local
   evidence reference. Preserve an omitted prior task ID.
5. Replace tested surfaces with current evidence/outcome, `fresh`, current
   snapshot, open finding IDs after closures, and `next_angle`.
6. Carry untested prior surfaces and remove closed IDs. Stale stays stale. Fresh
   stays fresh when spec path/digest and product digest are unchanged. Otherwise
   require one assessment: retain keeps fresh and stale marks stale. Assessments
   cannot name tested or unknown surfaces. Sort by unsigned UTF-8 `surface_key`.
7. Advance `next_finding_number` to one above the greatest report finding number
   at or above the prior counter; otherwise preserve it.
8. Reject more than 256 surface rows or a canonical candidate over 256 KiB; never
   trim unresolved findings or other rows.

Encode exact UTF-8 JSON using the index field order above, two-space indentation,
minimal decimal integers, standard JSON escaping equivalent to Go
`encoding/json` with `SetEscapeHTML(false)`, and exactly one final LF. Hash those
exact bytes for `index_sha256_after`, then discard the calculation-only candidate.
