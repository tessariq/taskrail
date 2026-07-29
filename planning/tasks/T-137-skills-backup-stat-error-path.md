---
id: T-137-skills-backup-stat-error-path
title: Report skills backupPath stat errors repo-relative
status: todo
priority: low
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies:
    - T-136-layout-marker-error-path-form
updated_at: "2026-07-29T09:50:03Z"
---

# T-137-skills-backup-stat-error-path Report skills backupPath stat errors repo-relative

## Description

Follow-up discovered while reviewing T-136-layout-marker-error-path-form.

`backupPath` in `internal/taskrail/skills.go:134` is the one remaining error site
in the package that names a file with its absolute path:

```go
return "", fmt.Errorf("stat backup %s: %w", candidate, err)
```

`candidate` is derived from `dest`, which is joined onto `s.paths.RepoRoot`, so
the message leaks the producer's absolute filesystem layout. It also wraps the
raw `err` instead of `fsCause(err)`, so the `*fs.PathError`'s own absolute path
is appended a second time — every sibling error in `installSkillFile`
(`skills.go:99`, `:102`, `:112`, `:117`) uses `relPath` plus `fsCause`.

Reached only when `init --with-skills --force` hits a `stat` failure other than
`ErrNotExist` on a backup candidate (e.g. a permission error on the skill
directory), so it is rare but adopter-facing.

## Acceptance

- `backupPath` reports the backup candidate repo-relative via `relPath` and
  unwraps the filesystem cause via `fsCause`, matching the sibling error sites in
  `installSkillFile`.
- Test coverage pins the path form so a regression fails, in the style of
  `TestLayoutMarkerReadErrorsReportRepoRelativePath`.
- No error wording change beyond the path form ("stat backup" stays).

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
