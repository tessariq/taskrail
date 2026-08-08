---
id: T-162-productize-digest-bound-post-spec-review-lenses
title: Productize digest-bound post-spec review lenses
status: todo
priority: high
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-215-add-the-generic-review-artifact-publisher
    - T-250-render-prompts-from-storage-neutral-context
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
updated_at: "2026-08-04T21:32:13Z"
---

# T-162-productize-digest-bound-post-spec-review-lenses Productize digest-bound post-spec review lenses

## Description

Turn consistency, gaps, additions, and adversarial spec review into independent
prompt and skill handoffs with strict JSON artifacts and one final digest-bound
disposition manifest.

## Acceptance

- Separate contexts independently emit fixed lens filenames with the exact
  schema-v1 fields, severity meanings, evidence, scope, open disposition, session
  identity, selected spec path, and role-mandated prompt source/template binding
  without receiving earlier conclusions as facts.
- The final manifest binds one final spec digest, every lens path/file
  digest/spec digest, and exactly one disposition per finding; strict decoding
  rejects unknown, null, duplicate, missing, or malformed data. Lens file digests
  bind prompt resolution transitively; the manifest does not duplicate it.
- The skill guides the human through every finding; the manifest alone records
  accepted/rejected/deferred decisions and rationale. Accepted findings name
  resulting headings, deferred findings name a future version, and unresolved
  high/medium findings forbid decomposition.
- Accepted spec edits are batched before any required rerun. Any spec byte edit
  stales all four lens reports and requires all four rerun against the final
  digest, while unchanged exact bytes never justify an additional lens round.
  Prompt-template drift stales the affected unpublished session and requires fresh
  lens observations rather than metadata repair; additions cannot silently expand
  scope.
- Final outputs publish through the generic review command with canonical
  no-follow, no-alias, absent-destination, same-directory atomic no-clobber
  behavior; lenses remain advisory and cannot invoke semantic writers or gate
  validate.
- The packaged review skill retains Agent Skills-compliant frontmatter; installed
  copies use nested `metadata.taskrail_version`, while marker-free committed
  copies remain byte-identical to the embedded package.

## Verification Notes

- Map criteria to exact-schema fixtures, independent-context metadata,
  built-in/replacement prompt bindings, publication race/alias cases, stale
  subject/template detection, complete dispositions, and forbidden-writer prompt
  mutations.
- Run all four lenses, edit one spec byte, prove the mixed snapshot cannot
  manifest, then rerun all four and approve final bytes.
- Run Agent Skills conformance and package-parity checks against the installed and
  committed review skill trees.

## Implementation Notes
