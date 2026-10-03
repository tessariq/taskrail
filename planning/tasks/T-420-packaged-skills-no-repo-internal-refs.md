---
id: T-420-packaged-skills-no-repo-internal-refs
title: Keep repository-internal task and spec references out of packaged skills
status: todo
priority: high
spec_ref: specs/v0.5.0.md#storage-neutral-packaged-skills
dependencies: []
updated_at: "2026-10-03T14:07:08Z"
---

# T-420-packaged-skills-no-repo-internal-refs Keep repository-internal task and spec references out of packaged skills


## Description

Packaged skills ship to every adopter through `taskrail init --with-skills`,
so their prose may cite only material an adopter has: Taskrail commands,
the adopter's own managed specs and tasks, and the skill's own references.
Today three packaged skills cite Taskrail's own development history, which
does not exist in an adopter repository:

- `taskrail-sdd-handoff/SKILL.md` stops on "semantic sizing under the T-251
  rubric".
- `taskrail-task-review/SKILL.md` checks "T-251 semantic sizing".
- `taskrail-gap/SKILL.md` names the `coverage --gaps` primitive "(T-100)" and
  justifies the semantic split with
  `specs/v0.4.0.md#recommendation-about-llm-support`.

An agent following these skills in an adopter repository either chases a
nonexistent task/spec or treats the sizing criterion as undefined. The
outcome is a self-contained packaged skill set: each such citation is
replaced by the content it stood for (the semantic sizing criteria stated
inline or in a shipped reference, the gap rationale stated in prose), and a
guard keeps Taskrail-repository task IDs and Taskrail's own versioned spec
paths from re-entering the package.

This is one outcome: the inline fixes and the guard deliver the same
guarantee and must not be split by skill file. It does not change skill
behavior, command surfaces, or the sizing rules themselves.

## Acceptance

- No file under `internal/taskrail/skills/` references a Taskrail-repository
  task ID (`T-<number>`) or a Taskrail product spec path
  (`specs/v<version>.md`); placeholder forms such as `T-<id>` and examples
  clearly framed as adopter content remain allowed.
- `taskrail-sdd-handoff` and `taskrail-task-review` state the semantic sizing
  criteria they apply (one independently meaningful outcome, split/do-not-split
  tests, no split by file/layer/phase/estimate) without depending on T-251.
- `taskrail-gap` explains why deep gap inference stays out of the binary
  without citing `specs/v0.4.0.md`.
- A package-level test fails when a repository-internal task ID or Taskrail
  spec path is added to any packaged skill file, and
  `TestSDDHandoffSkillRefusesAmbiguousEvidence` asserts the new wording.
- `task skills:regen` leaves `.agents/skills/` and `.claude/skills/`
  byte-identical to the package; `task check:skills` passes.

## Verification Notes

- Grep the package for `T-[0-9]` and `specs/v[0-9]`; expect only allowed
  placeholders.
- Add a deliberate `T-251` to a packaged skill in a scratch change; the new
  guard test fails; revert.
- Run `go test ./internal/taskrail` and `task check:skills`.
- Must land before v0.5.0 is released.

## Implementation Notes
