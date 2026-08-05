---
id: T-202-ship-the-lightweight-sdd-handoff-skill
title: Ship the lightweight SDD handoff skill
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#lightweight-sdd-handoff-skill
dependencies:
    - T-201-make-packaged-skills-agent-skills-compliant
updated_at: "2026-08-05T19:17:42Z"
---

# T-202-ship-the-lightweight-sdd-handoff-skill Ship the lightweight SDD handoff skill

## Description

Ship one provider-neutral `taskrail-sdd-handoff` skill that turns an operator-selected, already reviewed OpenSpec or Spec Kit artifact set into a conservative proposal for Taskrail's existing spec, import, and decomposition workflows. The handoff makes assumptions, unresolved decisions, semantic correspondences, and information loss visible before adoption. It stops when evidence is ambiguous and leaves all judgement-derived writes behind Taskrail's existing human review and digest-bound apply boundaries.

## Acceptance

- A1. The packaged addition is one `taskrail-sdd-handoff` skill with exactly two method references, `references/openspec.md` and `references/spec-kit.md`; each describes common artifact shapes, evidence to inspect, conservative Taskrail mappings, and known losses without claiming universal method compatibility.
- A2. The skill begins from an operator-selected local artifact set, reviews content rather than trusting directory names, generated templates, completion markers, or tool claims, and records assumptions and unresolved decisions in the proposed handoff.
- A3. Ambiguous approval, ownership, requirement meaning, task boundaries, dependencies, or target anchors causes the workflow to stop for operator review. The skill does not guess, silently omit uncertain material, or present ambiguity as a completed handoff.
- A4. A coherent product specification is routed through the existing `taskrail-spec` authoring and review flow; notes or structured task candidates are routed through `taskrail-import`; coverage and decomposition after Taskrail spec approval use `taskrail-decompose`. No parallel spec, task, lifecycle, review, or apply format is introduced.
- A5. Proposed task `spec_ref` values are checked against live headings in the selected local Taskrail spec. Proposed task bodies are outcome-focused, and imported or decomposition-generated tasks omit loop-policy fields and remain implicitly held.
- A6. The references state that the handoff does not prove provenance, approval, completeness, synchronization, change detection, round-trip fidelity, or continuing ownership of source artifacts.
- A7. The skill performs no automatic apply and adds no binary command or adapter, provider/model integration, source-system execution, provenance store, synchronization service, or format conversion owned by Taskrail core.
- A8. The embedded skill and both committed mirrors satisfy Agent Skills metadata rules and remain byte-identical under the package parity contract.

## Verification Notes

- A1, A6-A7: Static contract tests should assert the single skill identity, exact two-reference set, required exclusions, and absence of instructions for binary adapters, provider calls, synchronization, provenance claims, or automatic apply.
- A2-A5: Behavioral skill tests should present representative reviewed OpenSpec and Spec Kit artifacts and assert that the proposed next steps use the existing Taskrail flows, retain visible assumptions/losses, resolve only real local headings, and leave adoption unapplied.
- A3: Negative behavioral cases should include an artifact whose filename suggests approval but whose content does not, conflicting requirements, an unresolved target anchor, and incomplete task evidence; each must stop with an explicit operator decision rather than continue by inference.
- A4-A5: A sandbox CLI walkthrough should follow one coherent artifact set through spec review, import draft review, and decomposition preview, confirming no alternate format and no explicit loop policy are produced. Do not apply the proposal during this task's verification.
- A8: Run skill regeneration/parity and package validation, then task-body hygiene; retain the command outputs as evidence.

## Implementation Notes
