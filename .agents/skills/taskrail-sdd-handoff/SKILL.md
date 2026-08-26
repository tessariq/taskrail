---
name: taskrail-sdd-handoff
description: Propose a conservative handoff from reviewed OpenSpec or Spec Kit artifacts into existing Taskrail planning flows
---

# taskrail-sdd-handoff

Turn an operator-selected local artifact set, already reviewed in its source
method, into an advisory Taskrail handoff proposal. This skill is provider-neutral:
Taskrail does not inspect, execute, or synchronize the source method.

Read [OpenSpec guidance](references/openspec.md) or
[Spec Kit guidance](references/spec-kit.md) only after the operator identifies
which method produced the local artifacts. Neither reference claims universal
compatibility with every version, template, or extension of that method.

## Boundary

The handoff reviews artifact content rather than directory names, generated
templates, completion markers, or tool claims. It does not prove provenance,
approval, completeness, synchronization, change detection, round-trip fidelity,
or continuing ownership of source artifacts.

It makes no automatic apply. Do not create specs or tasks, invoke
`import --apply`, change lifecycle state, or publish review evidence from this
skill. It does not add a binary adapter, provider API, synchronization service,
provenance store, or format conversion to Taskrail core.

## Flow

1. **Establish the local inputs.** Record the operator-selected local artifact
   set, source method, intended Taskrail repository, and the stated review or
   approval evidence. Read the actual requirements, decisions, constraints,
   acceptance material, and task evidence. Treat a filename that suggests
   approval, a generated checklist, or a completed marker as a claim to inspect,
   not proof.
2. **Make the handoff visible.** Produce an advisory brief that separates:
   source evidence and its location; semantic correspondences to Taskrail
   concepts; assumptions; information losses; and unresolved decisions. Preserve
   conflicting or incomplete material in the brief rather than dropping it.
3. **Stop on ambiguity.** Stop for operator review when approval, ownership,
   conflicting requirements or requirement meaning, semantic sizing under the
   T-251 rubric (including whether to split or merge), integration ownership,
   dependencies, or target anchors are ambiguous, or when incomplete task
   evidence prevents a decision. Do not guess task boundaries, silently omit uncertain material,
   or call the handoff complete.
4. **Route by the coherent outcome.** A coherent product specification goes to
   the existing `taskrail-spec` authoring and review flow. Notes or structured
   task candidates go to `taskrail-import` for a reviewable import draft. Only
   after a Taskrail specification is approved, use `taskrail-decompose` for
   coverage and decomposition. These flows retain their own human review and
   digest-bound apply boundaries; this handoff introduces no parallel spec, task,
   lifecycle, review, or apply format.
5. **Resolve live anchors before proposing tasks.** For an existing local
   Taskrail specification, run `${TASKRAIL:-taskrail} spec show <version> --anchors
   --json` and use only returned headings for proposed `spec_ref` values. Stop for
   operator review when no live heading expresses the intended outcome. Proposed
   task bodies remain outcome-focused; imports and decomposition omit
   `loop_policy` and `loop_reason` and remain implicitly held.

## Handoff Brief

Keep the proposal concise and advisory. For each source item, identify its
content-based evidence, proposed Taskrail destination, assumptions or losses,
and any required operator decision. State which existing flow is appropriate and
what must be reviewed before a human chooses an apply step. A missing decision is
not an instruction to infer it later.

## Rules

- inspect source content, not source-system labels or completion claims
- stop for operator review on every ambiguity named above
- never claim the source method's approval or artifact status is mechanically
  verified by this handoff
- never automatically apply, create tasks, alter specs, or change task lifecycle
- never add a provider API, binary adapter, synchronization service, provenance
  store, or format conversion
- use only existing `taskrail-spec`, `taskrail-import`, and `taskrail-decompose`
  workflows for adoption
