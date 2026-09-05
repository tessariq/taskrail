#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
scanner="$script_dir/check-push-messages.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

# The fixture repository is built without hooks, which is exactly the history a
# `--no-verify` commit or a rebase produces.
repo="$tmp_dir/repo"
git init -q "$repo"
cd "$repo"
git config user.email fixture@example.com
git config user.name Fixture
git config commit.gpgsign false

commit() {
  printf '%s\n' "$2" >>file.txt
  git add file.txt
  git commit -q --no-verify -m "$1"
}

commit $'feat: add the first fact\n\nExplain the change.' one
clean_head="$(git rev-parse HEAD)"
if ! output="$(bash "$scanner" "$clean_head" 2>&1)"; then
  fail "a clean commit was rejected: $output"
fi

commit $'feat: add the second fact\n\nExplain the change.\n\nClaude-Session:\nhttps://claude.ai/code/session_01' two
if output="$(bash "$scanner" "$clean_head..HEAD" 2>&1)"; then
  fail "a session link was accepted"
fi
if [[ "$output" != *"automated-attribution"* ]]; then
  fail "the scanner did not name the policy: $output"
fi

# git hands the pre-push hook `<local ref> <local sha> <remote ref> <remote sha>`
# on stdin; the scanner reads that shape rather than guessing a range.
if printf 'refs/heads/main %s refs/heads/main %s\n' "$(git rev-parse HEAD)" "$clean_head" | bash "$scanner" >/dev/null 2>&1; then
  fail "the stdin range accepted a session link"
fi
if ! printf 'refs/heads/main %s refs/heads/main %s\n' "$clean_head" "$clean_head" | bash "$scanner" >/dev/null 2>&1; then
  fail "the stdin range rejected an empty push"
fi

# An agent identity in the author header makes the same claim the forbidden
# trailers make, so the push gate refuses it as well.
commit $'feat: add the third fact\n\nExplain the change.' three
agent_parent="$(git rev-parse HEAD~1)"
# --amend keeps the recorded author, so the identity is restated explicitly.
git commit -q --amend --no-verify --no-edit --author='Amp <amp@ampcode.com>'
if output="$(bash "$scanner" "$agent_parent..HEAD" 2>&1)"; then
  fail "an agent author was accepted"
fi
if [[ "$output" != *"agent identity"* ]]; then
  fail "the scanner did not name the author policy: $output"
fi

printf 'push message checks passed\n'
