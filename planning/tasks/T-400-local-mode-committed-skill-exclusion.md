---
id: T-400-local-mode-committed-skill-exclusion
title: Resolve local_exclusion_invalid on committed skill copies in local mode
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies: []
updated_at: "2026-09-11T16:28:44Z"
---

# T-400-local-mode-committed-skill-exclusion Resolve local_exclusion_invalid on committed skill copies in local mode

## Description

Diagnosed 2026-09-12; the product side is at fault and the fixture is not.

`localExclusions` (`internal/taskrail/local.go`) builds its managed set from the
marker, the local storage root, and every installed skill directory reported by
`InstalledSkillVersions` (`internal/taskrail/skills_version.go`). That reader
walks the embedded package against on-disk copies and never consults Git, so a
tracked copy and an ignored copy are indistinguishable to it. Each managed path
must then be both inside the managed exclude block and effectively gitignored.
A tracked file can never be gitignored, so a committed skill copy is an
unsatisfiable violation: the adopter can only clear it by un-committing the file
they deliberately committed.

Reproduced in a temp repository with no eval fixture involved: `init --local`,
commit one packaged `SKILL.md`, then `local status` reports one
`local_exclusion_invalid` over `.agents/skills/taskrail-repair` with
`promotion_ready: false`. The control case is clean — `init --local
--with-skills` installs the copies ignored and reports no violation. The
evaluation sighting came from the harness committing its `.agents/skills` copy
into the sandbox, which is T-411's subject, not a registry defect.

Committing packaged skills is the supported shape, not a misconfiguration: this
repository commits `.agents/skills/` and `.claude/skills/` and guards them with
a parity check. Local mode's promise covers Taskrail's own planning state, not
what the adopter chooses to commit. So the managed set becomes Git-aware: a
skill copy tracked by Git belongs to the adopter and is not a managed local
path, while an untracked copy remains managed and must stay excluded. Asking Git
also keeps the answer in the authoritative place rather than in new persisted
provenance state.

Outcome: a local-mode repository that commits its packaged skill copies reports
no exclusion violation, while ignored copies are still required to stay
excluded.

## Acceptance

- A skill copy tracked by Git is not a managed local path: it produces no
  `local_exclusion_invalid`, is absent from the managed exclusion rows, and does
  not clear `promotion_ready`. "Tracked" means present in `HEAD` or in the index,
  so a staged-but-uncommitted copy counts as the adopter's and a copy removed
  with `git rm --cached` returns to managed.
- An untracked skill copy keeps today's behavior exactly: managed, required to be
  both inside the managed exclude block and effectively gitignored, and reported
  as `local_exclusion_invalid` when it is not.
- The marker and the local storage root stay managed unconditionally; this change
  narrows only the skill-directory contribution to the managed set.
- A repository mixing tracked and untracked skill copies reports each on its own
  terms in one `local status` run.
- `local promote --with-skills` agrees with the narrowed set: an already-tracked
  copy is left untouched and is neither rewritten nor reported as promoted, and
  promote and status never disagree about whether a given copy is managed.
- `local status` stays read-only and advisory, and `validate` is unaffected in
  every case above.

## Verification Notes

- Temp-repo table test over `local status` for: committed copy, untracked
  ignored copy, staged-only copy, `git rm --cached` copy, and a mixed
  repository; assert violations, exclusion rows, and `promotion_ready`.
- Reproduction recorded above used `init --local` plus one committed
  `.agents/skills/taskrail-repair/SKILL.md`; keep that as the regression case.
- Manual test `local promote --with-skills` against a repository with one
  tracked and one untracked copy, since this alters visible workflow behavior.

## Implementation Notes
