#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
checker="$script_dir/check-commit-msg.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_accepts() {
  local name="$1"
  local message="$2"
  local message_file="$tmp_dir/$name"
  local output

  printf '%s\n' "$message" >"$message_file"
  if ! output="$("$checker" "$message_file" 2>&1)"; then
    fail "$name was rejected: $output"
  fi
}

assert_rejects() {
  local name="$1"
  local message="$2"
  local expected="$3"
  local message_file="$tmp_dir/$name"
  local output

  printf '%s\n' "$message" >"$message_file"
  if output="$("$checker" "$message_file" 2>&1)"; then
    fail "$name was accepted"
  fi
  if [[ "$output" != *"$expected"* ]]; then
    fail "$name did not report '$expected': $output"
  fi
}

body_72="$(printf '%072d' 0)"
body_73="$(printf '%073d' 0)"

assert_accepts conventional $'docs: clarify contributor workflow\n\nExplain why contributors need the clarified path.'
assert_rejects unknown-type $'build: configure tooling\n\nTaskrail does not list build among its Conventional Commit types.' 'Conventional Commit'
assert_accepts task-suffix $'feat(domain): add scope (T-001)\n\nKeep repository matching in the domain layer.'
assert_accepts generated-merge "Merge branch 'feature'"
assert_accepts generated-revert 'Revert "feat: add scope (T-001)"'
assert_accepts body-at-72 "test: accept bounded body lines

$body_72"

assert_rejects missing-body 'docs: reject missing body' 'descriptive body'
assert_rejects unseparated-body $'docs: reject body\nExplain why.' 'descriptive body'
assert_rejects body-over-72 "test: reject long body lines

$body_73" '72 characters'
assert_rejects slugged-task $'feat: add scope (T-001-scope)\n\nExplain the change.' 'task references'
assert_rejects prefixed-task $'feat: T-001 add scope\n\nExplain the change.' 'task references'
assert_rejects invalid-conventional $'add scope\n\nExplain the change.' 'Conventional Commit'
assert_rejects attribution $'feat: add scope\n\nExplain it.\n\nCo-Authored-By: Bot <bot@example.com>' 'automated-attribution'
assert_rejects agent-session-trailer $'feat: add scope\n\nExplain it.\n\nClaude-Session: https://claude.ai/code/session_01' 'automated-attribution'
assert_rejects wrapped-session-link $'feat: add scope\n\nExplain it.\n\nClaude-Session:\nhttps://claude.ai/code/session_01' 'automated-attribution'
assert_rejects thread-trailer $'feat: add scope\n\nExplain it.\n\nAmp-Thread: https://ampcode.com/threads/T-1' 'automated-attribution'
assert_rejects thread-id-trailer $'feat: add scope\n\nExplain it.\n\nAmp-Thread-ID: T-01a0' 'automated-attribution'
assert_rejects lowercase-coauthor $'feat: add scope\n\nExplain it.\n\nCo-authored-by: Someone <a@b.c>' 'automated-attribution'
assert_accepts generated-in-prose $'docs: describe the generated fixtures\n\nThe fixtures are generated with the recorded payloads.'

printf 'commit message checks passed\n'
