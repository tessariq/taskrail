# Skills Productization Decisions

Contract for how Taskrail's tracked-work skills become a supported product
surface. Records the three decisions the v0.2.0 spec
(`specs/v0.2.0.md#agent-workflow-skills`) requires be made explicitly, with
rationale and non-goals. No runtime behavior lives here; this document gates the
implementation tasks T-029, T-030, and T-034.

## Decision 1: Portability

Shipped skills must be repo-agnostic. They invoke the installed `taskrail`
binary, never `go run ./cmd/taskrail`, which only resolves inside the Taskrail
source tree.

Rationale: adopting repositories have no Go module for Taskrail and no
`./cmd/taskrail` package. A skill that shells out to `go run` cannot run in any
repository other than this one.

Since T-055 this is the only rule: the bespoke `skills/` source was retired and
this repository adopts the packaged set like any adopter, invoking the binary via
the configurable entry point `${TASKRAIL:-taskrail}` (T-051). The dogfooding-vs-
shipped portability split no longer exists.

## Decision 2: Distribution Mechanism

Exactly one initial path: **embedded skill files via `embed.FS`, written only on
explicit opt-in.** Skills are embedded in the binary and materialized only when
the user runs `taskrail init --with-skills`.

The default `taskrail init` must not write `.agents/` or `.claude/` skill
directories, or any other agent-tool directory. Provisioning agent-tool
directories is opt-in and never silent.

Rationale: embedding keeps the shipped skill text versioned with the binary that
implements the commands the skills call, so the two cannot drift. Gating on an
explicit `--with-skills` flag keeps `taskrail init` minimal and avoids writing
provider-specific directories into repositories that do not want them.

Rejected alternative:

- Documentation-only (adopters copy skill text manually): loses version
  coupling between skills and the binary; higher adoption friction.

Note: writing skills on default `taskrail init` was never an option — it is
ruled out by the constraint above, not a considered alternative.

Materialized skill files carry `metadata.taskrail_version` recording the binary
that wrote them, because a non-destructive install means upgrading the binary
never refreshes an existing on-disk copy. Readers accept the legacy top-level
`taskrail_version` field from released v0.4 binaries; an explicit successful
refresh normalizes that legacy form to nested metadata.

This repository's committed `.agents/`/`.claude/` copies are regenerated from the
embedded package by `task skills:regen`, which copies the package source and
therefore carries no marker — that is what keeps `task check:skills` parity
byte-exact against a version the ldflags-injected binary would otherwise vary.
Do not regenerate the committed copies with `init --with-skills`; the stamped
output would fail parity.

Because the committed copies carry no marker, they were once reported on every
command as unknown-version (T-120) — a standing stderr line no contributor could
clear. Since T-124 the skew check exempts them: a marker-free skill that is
byte-identical to the package embedded in the running binary was copied from that
package, not installed by an older binary, so there is no version to be skewed
against and the check stays silent. Regenerating with `task skills:regen` keeps
that property.

The exemption is parity-only, so it cannot hide a real problem. Edit a committed
copy without regenerating and it diverges from the package: it reports as
unknown-version again — and `task check:skills` fails, which is the real signal.
Do not "resolve" an unknown-version line with `taskrail init --with-skills
--force`; that stamps the committed copies and breaks parity. Adopters, whose
copies are installed rather than parity-checked, see the `--force` remedy only
when a real version mismatch is readable.

## Decision 3: Relationship To Task-Creation Ergonomics

Shipped skills call the real `taskrail task new` command (see T-027 / T-028)
instead of hand-authoring task markdown.

Rationale: today's dogfooding skills compensate for the absence of a
task-creation command by writing task files by hand. That duplicates the task
schema inside skill text and drifts from the CLI's own validation. A real
command lets a skill create a well-formed task with one non-interactive call, so
skill text carries workflow, not schema.

The task-creation ergonomics work and the skills work are designed together: as
`taskrail task new` absorbs schema responsibility, shipped skills shrink to
workflow orchestration.

## Non-Goals

Taskrail distributes skills as static, provider-agnostic text. It does **not**:

- execute skills,
- schedule skills, or
- orchestrate skills.

Running a skill remains the agent's responsibility, consistent with the LLM and
runtime exclusions in the spec. There is no skill-execution, skill-scheduling,
or skill-orchestration runtime in Taskrail.

Maintainer behavioral evals are likewise not part of the embedded package. They
may execute skills through caller-owned agents during release review, but Taskrail
does not choose a provider, carry credentials, judge a model run inside the
binary, or apply an eval-generated skill patch automatically.

## The Packaged Skill Set

Input list for T-029, which owned the final selection and the portability
rewrite. Historically this table split canonical dogfooding skills (`skills/`)
from the shipped set; T-055 retired that split, so every row below is simply part
of the one packaged set under `internal/taskrail/skills/`, materialized to the
committed `.agents/skills/` and `.claude/skills/` copies:

| Skill | Disposition | Reason |
|-------|-------------|--------|
| `autonomous-backlog` | Shippable | Generic tracked-work cycle (validate, select, start, implement, verify, follow-up); no repo-local assumptions once `go run` is replaced by the installed binary. |
| `autonomous-task` | Shippable | Executes one selected task through CLI transitions; portable after the binary rewrite. |
| `autonomous-verify` | Shippable | Drives `taskrail verify` against acceptance criteria and points at product-level verification artifacts. |
| `autonomous-recovery` | Shippable | Routes every correction through `taskrail repair` (dry-run -> apply -> re-validate) and never hand-edits authoritative state. Shipped in T-054 once the widened repair surface (T-072) shrank the human-resolved residue to what the skill claims; the earlier "falls back to manual edits" premise is stale. |
| `autonomous-manual-test` | Shippable | Guides manual testing against task acceptance criteria; repo-agnostic after the binary rewrite. Shipped in T-081 **without** promoting `planning/artifacts/manual-test/` to a product invariant: its artifacts stay ephemeral and gitignored, `init` does not provision the directory, and `validate` stays unaware of it. This resolves the v0.2.0 "Artifact And Init Consistency" deferral in favor of shipping the skill rather than adding an invariant. |
| `taskrail-loop` | Shippable | Interactively configures and supervises one bounded provider-neutral `loop` invocation. It confirms a structured dry-run, reads the terminal result file, and reports coordinator or caller-owned adapter evidence without gaining selection, lifecycle, Git, integration, delivery, or recovery-write authority (T-337). |
| `taskrail-repair` | Shippable | Drives the conservative `taskrail repair` loop (dry-run -> apply -> re-validate) to reconcile mechanical `STATE.md` drift; repo-agnostic and never hand-edits authoritative state (T-050). |
| `taskrail-spec` | Shippable | Inspects and authors specs through the `taskrail spec` command family and anchors tracked work to real `spec_ref` headings; repo-agnostic (T-064). |
| `taskrail-spec-review` | Shippable | Stages four independent advisory post-spec lens observations and publishes one human-dispositioned digest-bound bundle before decomposition; it performs no semantic writes (T-162). |
| `taskrail-decompose` | Shippable | Authors strict ImportDraft v2 task bodies, performs at most two fresh-context adversarial passes, publishes one manifest-bound bundle, and applies its exact digests; spec-driven and provider-neutral (T-098, T-304). |
| `taskrail-gap` | Shippable | Composes `coverage --gaps --json` (structural candidates) with agent semantic gap review over covered active-spec areas, proposing tasks a human promotes via `task new` / `import --apply`; advisory-only, adds no binary surface (T-101). |
| `taskrail-task-review` | Shippable | Reviews one existing task as a strict digest-bound advisory snapshot, then publishes it through `review publish --type task`; accepted changes remain human-routed through existing task authoring, exact-ID dependency editing, or reviewed task production (T-216). |
| `taskrail-workflow-adversarial` | Shippable | Runs one bounded post-implementation review in an isolated sandbox and publishes only the strict Git/spec/product-bound report plus Taskrail-derived serial memory; it has no product, lifecycle, or finding-promotion authority (T-306). |

T-029 may revise this list, but must justify any change against the three
decisions above.

## Onboarding Skills

The onboarding skills target a repository that is not yet Taskrail-managed, which
Taskrail's own already-managed repository never exercises. They live in the same
packaged set (`internal/taskrail/skills/`) as the tracked-work skills and honor the
same three decisions — repo-agnostic,
installed-via-`--with-skills`, and task-creation through a real command rather
than hand-authored markdown.

| Skill | Origin | Task creation | Reason |
|-------|--------|---------------|--------|
| `taskrail-import` | T-034 | `taskrail import --apply` | Turns markdown notes/drafts into spec and task files via the agent-in-the-loop import path; the binary stays LLM-free. |
| `taskrail-retrofit` | T-043 | `taskrail import --apply` | Drives the guided retrofit bootstrap (detect -> dry-run -> confirm -> apply -> adopt -> validate) for an existing repo, adopting reviewed notes as tracked work through the import pipeline (T-042). |

## Cross-References

- Spec: `specs/v0.2.0.md#agent-workflow-skills`
- Skill catalog and packaging: `docs/workflow/skills-overview.md`
- Downstream implementation tasks: T-029 (shippable skill selection and
  portability rewrite), T-030 (`init --with-skills` distribution), T-034
  (`taskrail-import` skill), T-043 (`taskrail-retrofit` skill).
