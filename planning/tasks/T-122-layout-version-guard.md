---
id: T-122-layout-version-guard
title: Enforce layout_version compatibility in every command, not only init
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#layout-compatibility-beyond-init
dependencies: []
updated_at: "2026-07-27T13:49:41Z"
---

# T-122-layout-version-guard Enforce layout_version compatibility in every command, not only init

## Description

`init` refuses a repository whose marker records a `layout_version` newer than the
running binary supports — "repository layout_version %d is newer than supported %d;
upgrade taskrail" — so an older CLI never mangles a layout it does not understand.
Grepping every use of `LayoutVersion` shows that comparison lives only in
`init.go`: `start`, `verify`, `complete`, `block`, `repair`, and the read-only
reporting commands all load the layout through `loadLayoutConfig` and proceed
without ever checking the version.

Today that is latent — `currentLayoutVersion` is still 1, so no repository can be
newer — but the guard is missing from precisely the commands that write, and it will
matter the first time the layout advances. Fixing it before there is a version 2 is
much cheaper than fixing it after a version-2 repository has been written to by a
version-1 binary.

## Acceptance

- A marker recording a `layout_version` newer than the binary supports is refused by
  any command that loads the layout, using the existing wording, **before** any
  read-modify-write of state or task files.
- The check lives in one shared place that every command reaches through the normal
  layout load, rather than being repeated per command — the two must not be able to
  drift.
- Read-only commands (`validate`, `status`, `stats`, `coverage`) refuse too: emitting
  a plausible-looking report against a layout the binary cannot model is worse than
  refusing.
- An equal or older `layout_version` is unaffected: older still migrates through
  `init`, and every current repository behaves exactly as before.
- Covered by a test that writes a marker with a future `layout_version` and asserts
  each command category refuses without writing, plus one asserting the current
  version is untouched.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
