---
name: taskrail-sdd-handoff-openspec-reference
description: Conservative OpenSpec artifact mapping guidance for the Taskrail SDD handoff skill
---

# OpenSpec Handoff Reference

OpenSpec repositories commonly organize a proposed change around a change name
with a proposal, requirement deltas, design notes, and task or checklist material.
Names and layouts vary. Inspect the actual prose for the requested behavior,
accepted scope, constraints, dependencies, ownership, and evidence; a `complete`
status, generated template, or directory name is not approval evidence by itself.

Map a coherent, reviewed behavior and its constraints to a proposed Taskrail spec
area. Map notes or candidate work only to a reviewable `taskrail-import` handoff.
After a Taskrail spec is approved, map uncovered approved requirements to the
existing `taskrail-decompose` flow. Preserve source-to-Taskrail correspondences,
assumptions, unresolved decisions, and losses in the advisory brief.

OpenSpec source artifacts can express deltas, not a complete product spec. Their
change status, source ownership, review history, baseline selection, task order,
and local conventions may be unavailable or incompatible with Taskrail. This
handoff does not prove provenance, approval, completeness, synchronization,
change detection, round-trip fidelity, or continuing ownership of source
artifacts. It is not a universal OpenSpec conversion.
