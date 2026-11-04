---
id: T-451-clarify-released-v04-import-body-limitations
title: Clarify released v0.4 import body limitations
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies: []
updated_at: "2026-11-04T22:29:10Z"
---

# T-451-clarify-released-v04-import-body-limitations Clarify released v0.4 import body limitations

## Description

Make the released v0.4 import body's limitations explicit to operators without
changing schema-v1 publication behavior. In v0.4.0, the emitted prompt calls
task.body "markdown detail for the task file" and the import contract describes
it as a task Markdown body, but apply succeeds with TODO scaffolding instead.
The released createDraftTasks path never passes the supplied body to CreateTask.

The original fmueller/latinitas-cards report supplied detailed bodies for
T-184 through T-205. A separate disposable-repository reproduction using the
mise-pinned v0.4.0 binary returned Body preserved: False and Scaffold present:
True. This is a Taskrail release-contract mismatch, not a consumer lifecycle bug.

Current main at 02178eb deliberately retains scaffold-only v1 imports. T-304
(9bb06e18a9eabf305d2723d5467fe42d5648fb58) corrected the prompt and pinned that
behavior. Reviewed v2 exact-body publication was introduced by T-163
(6b39f8333dcb40ed42b91da2e56ea4789e87052e). GitHub listed v0.4.0 as the latest
release during this investigation, leaving pinned consumers on the misleading
advertisement despite the main-branch correction.

Prepare a narrowly scoped 0.4.x compatibility clarification for maintainer
review, or record the maintainer's explicit decision to deliver it only in the
next planned release. Do not backport all of T-304 or the reviewed-v2 subsystem.
T-421 separately owns new apply-time body-discard diagnostics; neither task
requires the other. No Latinitas Cards files or task lifecycle may be changed.

## Acceptance

- A1. Release-targeted prompt and documentation consistently state that v1
  accepts but ignores task.body and creates the standard scaffold. Release
  notes identify the v0.4.0 advertised-contract mismatch and explain that the
  clarification does not restore previously omitted bodies or add preservation.
- A2. Retain v1 schema, successful scaffold-only apply, metadata, dependencies,
  and lifecycle behavior. Do not add T-421's warnings or enable v2 on 0.4.x.
  Distinguish current reviewed-v2 preservation from released-v0.4 capabilities:
  v2 requires a supported binary, manifest-bound publication, and exact draft
  and manifest digests; changing schema_version alone is not a migration.
- A3. Obtain and record the maintainer's delivery decision: a minimal 0.4.x
  clarification patch or inclusion in the next planned release. Prepare the
  corresponding reviewable changes without tagging, publishing, modifying
  consumer version pins, or claiming an unshipped correction is available.

## Verification Notes

- A1: compare the v0.4.0 emitted prompt and import documentation with the chosen
  release candidate. Focused prompt coverage must detect the misleading old
  promise and require an explicit ignored-body/scaffold explanation.
- A2: in a disposable Git repository, initialize with the release-target binary
  and apply a v1 draft containing a distinctive Markdown body and dependency.
  Confirm success, scaffold content, absence of that body, and unchanged
  metadata/dependency semantics. Persist a sandbox manual-test plan and report;
  remove the disposable repository afterward.
- A2: on main, the investigation executed and passed
  TestApplyImportDraftV1IgnoresLegacyBodyAndUsesStandardScaffold,
  TestApplyImportDraftV2PublishesExactReviewedBodies, and
  TestEmitImportPromptDefinesV1OutcomeScaffoldAndImplicitHold using
  go test ./internal/taskrail -run '<exact test name>' -count=1 -v.
  Retain the relevant regressions for the selected release target; do not
  claim that reviewed v2 is supported by v0.4.0.
- A3: record the chosen target and approval in implementation notes. Inspect
  the final diff for a clarification-only scope and check the release guidance;
  package publication requires separate explicit authorization.

### Investigation Sources

- Original consumer report:
  https://ampcode.com/threads/T-01a19e80-54a8-74e8-9c51-4ba760ebfad5
- Independent v0.4.0 reproduction:
  https://ampcode.com/threads/T-01a1a04d-6d8e-7530-8cf1-94952d8ce669
- Taskrail contract verification and passing regression results:
  https://ampcode.com/threads/T-01a1a148-b069-7220-b972-4af7703db41c

## Implementation Notes
