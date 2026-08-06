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

## Version Intent

- `v0.1.0` proves the repo contract, deterministic task progression, explicit state continuity, and verification artifacts.
- `v0.2.0` adds retrofit and import ergonomics.
- `v0.3.0` explores spec-task coverage and drift detection.
- `v0.4.0` adds slugged task authoring and re-slug/rename ergonomics, and carries the deferred spec-to-task decomposition and gap-analysis threads.
- `v0.5.0` adds autonomous workflow integrity and reviewable planning: a uniform versioned agent JSON API, repository locking and recoverable batch adoption, Agent Skills-compliant lifecycle-complete skills and prompts, human-owned repository notes, outcome-focused task authoring and review, exact-ID dependency editing, safe review publication, a bounded provider-independent loop with task-local policy and machine-readable outcomes, lightweight spec-driven-development handoffs, post-spec and workflow-adversarial review, adversarial task decomposition, and maintainer-run skill evaluations.
- `v0.6.0` adds durable arbitrary-width and opaque task references, first-class cancellation with preview, stable-reference dependency editing, durable legacy imports, and explicit immutable task archival over one committed-or-local live-plus-archive ledger.
- `v0.7.0` adds digest-bound planning-source interoperability through strict built-in OpenSpec and Spec Kit profiles, reviewed committed or private-local spec mapping with ImportDraft v3, trust-labelled immutable provenance receipts, and read-only profile/receipt inventories.
