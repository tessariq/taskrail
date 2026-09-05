#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
setup="$repo_root/.agents/setup"
resume="$repo_root/.agents/resume"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

for hook in "$setup" "$resume"; do
  [ -x "$hook" ] || fail "${hook#"$repo_root/"} must exist and be executable"
  bash -n "$hook" || fail "${hook#"$repo_root/"} has invalid shell syntax"
done

grep -qF 'mise run setup' "$setup" || fail '.agents/setup must run the repository mise setup task'
grep -qF 'core.hooksPath' "$setup" || fail '.agents/setup must activate repository-local hooks'
grep -qF 'lefthook install' "$resume" || fail '.agents/resume must repair the Lefthook installation'
grep -qF 'core.hooksPath' "$resume" || fail '.agents/resume must repair the repository-local hook path'

printf 'orb setup checks passed\n'
