---
name: taskrail-task-review
description: Review one existing Taskrail task against its spec without directly mutating tracked work
---

# taskrail-task-review

Review one existing task as an advisory, digest-bound snapshot. The binary stays
LLM-free: it resolves, validates, and publishes bytes; you make the semantic
judgement. This is not implementation review, verification, mechanical gap
analysis, or a second family of the four post-spec review lenses.

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

## Review Flow

1. **Resolve one task.** Run `${TASKRAIL:-taskrail} task show <task-id> --json`.
   Require the exact full persisted ID. Consume the returned `task_id`,
   `task_path`, content, and `sha256`; do not open the logical task path
   directly. Parse its `spec_ref`, status, and exact dependency IDs from the
   returned content. Run `${TASKRAIL:-taskrail} spec show <version> --json` for
   the referenced version and consume its exact content and SHA-256 result.
2. **Inventory related context.** Inspect every declared dependency and other
   relevant full task ID through `${TASKRAIL:-taskrail} task show <task-id>
   --json`. When the selected task's referenced spec is active, run
   `${TASKRAIL:-taskrail} coverage --area <spec-anchor> --json` to identify its
   linked same-area tasks. For a historical or future spec, record the exact
   related IDs available from the selected task, human-provided context, and
   durable review references; do not require active-spec coverage. Identify only
   repository contracts material to the proposed outcome; never substitute a
   filesystem scan of managed logical paths for these commands.
3. **Stage one proposal.** Choose a portable lowercase session key and an absent
   destination `<planning-dir>/reviews/task/<task-id>/<session-id>/`. Obtain the
   current transient artifacts root from `${TASKRAIL:-taskrail} status --json`;
   stage only `<proposal>/review.json` beneath its ignored
   `review-proposals/task/<session-id>/` directory. Render the role-mandated
   instructions with `${TASKRAIL:-taskrail} prompt render task-review --task
   <task-id> --review <proposal>/review.json --json`. Use the resolved prompt's
   `source` and `template_sha256`, task/spec paths and SHA-256 values, and the
   rendered instructions to produce exactly one `review.json`. Do not add a
   summary file, transcript, manifest, or any other proposal member.
4. **Judge the boundary.** Check outcome/spec alignment, T-251 semantic sizing,
   overlap, dependency direction, integration ownership, acceptance, negative
   boundaries, evidence/oracles, operator gates, and unnecessary implementation
   prescription. Apply both split and do-not-split tests. A sizing finding must
   name whether body clarification, an edge correction, a reviewed split/merge,
   or a new outcome is appropriate, and which task owns integrated delivery.
5. **Preview and publish.** Run `${TASKRAIL:-taskrail} review publish --type task
   --proposal <proposal> --destination <destination> --task <task-id>
   --expect-task-sha256 <digest> --expect-spec-sha256 <digest> --dry-run --json`.
   Consume validation and subjects. If it is valid and the destination remains
   absent, publish the same candidate with `${TASKRAIL:-taskrail} review publish
   --type task --proposal <proposal> --destination <destination> --task <task-id>
   --expect-task-sha256 <digest> --expect-spec-sha256 <digest> --json`. The
   publisher rechecks task, spec, proposal, configuration, and resolved prompt
   bytes under its writer lock; source drift requires a new observation, never
   metadata repair. Then inspect the durable bytes with `${TASKRAIL:-taskrail}
   review show <destination>/review.json --json`.

## Strict Proposal Shape

The proposal contains exactly one `review.json`. It is schema v1 with exactly
these ordered top-level fields: `schema_version`, `prompt_id`,
`prompt_contract_version`, `prompt_template_sha256`, `prompt_source`,
`session_id`, `task_id`, `task_path`, `task_sha256`, `spec_path`, `spec_sha256`,
`context_mode`, `generated_at`, and non-null `findings`. Set the prompt binding
to the rendered `task-review` v1 resolution, use lower-case raw-byte SHA-256
values, canonical repository-relative logical paths, and canonical RFC3339 UTC.
The session ID equals the destination basename. Every finding has exactly
`finding_id`, `severity`, `evidence`, `impact`, `recommendation`,
`disposition`, and `rationale`; textual fields are non-empty and IDs are unique.

## Remediation Boundary

This skill is advisory and never changes task status, task-local loop policy,
specs, dependencies, or task bodies directly.

- Accepted body-only clarification is a human-approved, digest-bound
  `${TASKRAIL:-taskrail} task author <task-id> --body <proposal.md>
  --expect-sha256 <digest> --json` handoff. Only `todo` tasks may be authored.
- An accepted one-edge correction is a human-approved exact-ID
  `${TASKRAIL:-taskrail} task dependency add <task-id> <dependency-id> --json`
  or `${TASKRAIL:-taskrail} task dependency remove <task-id> <dependency-id>
  --json` handoff.
- A genuine new outcome, split, or consolidation routes through a reviewed
  task-producing flow. Create any new work as a reviewed implicit-hold follow-up;
  do not derive unattended authority from its body text.

Todo tasks may continue into authoring. Active, blocked, completed, and
cancelled tasks remain reviewable but are not authored through this skill.
A later authored-body change starts another review only when an explicitly invoked consuming workflow or the human requires final-byte review. For
unchanged bytes or confidence-seeking alone, do not start another review session.

## Rules

- never hand-edit `planning/STATE.md`, task frontmatter, task bodies, or status
  fields
- stage untrusted proposal bytes only under the ignored artifacts root; final
  evidence is published only by `review publish`
- do not claim that prompt binding proves reviewer identity, delivery, or
  independence
- do not clone the post-spec consistency, gaps, additions, or adversarial lenses
- do not mutate an accepted finding yourself; name its sanctioned human-invoked
  route and preserve the published review as the immutable snapshot
