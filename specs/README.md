# Specs

`specs/` contains versioned product specifications for Taskrail itself.

## Reading Order

1. `specs/v0.1.0.md`
2. `specs/v0.2.0.md`
3. `specs/v0.3.0.md`
4. `specs/v0.4.0.md`
5. `specs/v0.5.0.md`
   - Co-normative machine contract: `specs/contracts/v0.5.0-machine-api.md`
6. `specs/v0.6.0.md`
7. `specs/v0.7.0.md`

## Rules

- Specs are normative.
- `README.md` and workflow docs are orientation material, not the authoritative product definition.
- Tasks under `planning/tasks/` must link to one or more live headings in the relevant spec file.
- `planning/STATE.md` declares the active spec version and active spec path.
- A spec version stays near the size of `specs/v0.4.0.md` (about 450 lines).
  Test-requirement inventories, byte-level schemas, and field-order rules belong
  in code, golden fixtures, and `docs/`, not in the normative spec. Going past
  the ceiling needs a written reason in the spec's `## Summary`, because spec
  size drives task count, test volume, and release-gate size downstream
  (`v0.5.0` grew to four times `v0.4.0` and its gate restarted three times).

## Version Intent

- `v0.1.0` proves the repo contract, deterministic task progression, explicit state continuity, and verification artifacts.
- `v0.2.0` adds retrofit and import ergonomics.
- `v0.3.0` explores spec-task coverage and drift detection.
- `v0.4.0` adds slugged task authoring and re-slug/rename ergonomics, and carries the deferred spec-to-task decomposition and gap-analysis threads.
- `v0.5.0` adds autonomous workflow integrity and reviewable planning: a uniform versioned agent JSON API, repository locking and recoverable batch adoption, Agent Skills-compliant lifecycle-complete skills and configurable prompts, human-owned repository notes, outcome-focused task authoring and review, prompt-bound safe review publication, a bounded provider-independent loop with task-local policy and machine-readable outcomes, lightweight spec-driven-development handoffs, post-spec and workflow-adversarial review, adversarial task decomposition, and maintainer-run skill evaluations.
- `v0.6.0` adds durable arbitrary-width and opaque task references, first-class cancellation with preview, stable-reference dependency editing, durable legacy imports, explicit immutable task archival over one committed-or-local live-plus-archive ledger, guided active-spec transitions and release readiness, embedded skill inspection, and explicit agent-mode JSON/help output.
- `v0.7.0` adds digest-bound planning-source interoperability through strict built-in OpenSpec and Spec Kit profiles, reviewed committed or private-local spec mapping with ImportDraft v3, trust-labelled immutable provenance receipts, and read-only profile/receipt inventories.
