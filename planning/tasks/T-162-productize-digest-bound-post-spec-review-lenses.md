---
id: T-162-productize-digest-bound-post-spec-review-lenses
title: Productize digest-bound post-spec review lenses
status: completed
priority: high
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-299-bind-spec-review-publication-to-resolved-prompts
updated_at: "2026-08-26T08:01:52Z"
completion_id: "3e3ca7039309dc5a2ff402733b8c71e4"
last_verification_id: "1cc87e7dc6ffdd36473b74bf67cc8a54"
last_verification_result: pass
last_verified_at: "2026-08-26T08:01:52Z"
last_verified_completion_id: "3e3ca7039309dc5a2ff402733b8c71e4"
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
- Lenses flag sizing only when spec prose itself prevents coherent decomposition,
  such as inseparable outcomes, contradictory boundaries, or missing integration
  ownership. They do not size proposed or existing tasks or duplicate the T-251,
  decomposition, or task-review judgments.
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
- Include ambiguous spec-boundary and already-decomposable fixtures proving the
  lenses report only prose-level decomposition blockers, not task-size findings.
- Run all four lenses, edit one spec byte, prove the mixed snapshot cannot
  manifest, then rerun all four and approve final bytes.
- Run Agent Skills conformance and package-parity checks against the installed and
  committed review skill trees.

## Implementation Notes

- 2026-08-26T08:01:52Z: Productized independent digest-bound post-spec review prompts and packaged skill guidance.
- 2026-08-26T08:01:52Z: verification pass id 1cc87e7dc6ffdd36473b74bf67cc8a54 previous none completion 3e3ca7039309dc5a2ff402733b8c71e4
