---
id: T-152-require-exact-goreleaser-version-ldflag
title: Require exact GoReleaser version ldflag
status: todo
priority: low
spec_ref: specs/v0.4.0.md#goals
dependencies:
    - T-149-harden-release-version-and-changelog-notes-guards
updated_at: "2026-07-29T17:30:44Z"
---

# T-152-require-exact-goreleaser-version-ldflag Require exact GoReleaser version ldflag

## Description

The GoReleaser version guard currently accepts any ldflag containing
`-X main.version=v{{.Version}}`. A malformed value with an appended suffix, such
as `v{{.Version}}-dirty`, therefore passes even though release binaries would
violate the exact `v<version>` contract in `specs/v0.4.0.md#goals`. Tighten the
test without changing the release configuration's intended value.

Follow-up derived from T-149-harden-release-version-and-changelog-notes-guards's verification or discovery.

## Acceptance

- The repository guard requires the complete `main.version` assignment to equal
  `v{{.Version}}`, not merely contain that text.
- A deliberate suffix or prefix regression in the ldflag makes the guard fail.
- The current GoReleaser configuration passes the strengthened test.
- The assertion remains resilient to unrelated linker flags such as `-s` and `-w`.

## Verification Notes

- Review finding from T-149 identified the substring assertion in
  `internal/toolchain/release_test.go` as permissive.

## Implementation Notes
