#!/usr/bin/env bash
# commit-msg hook: enforce a Conventional Commit subject and reject
# automated-attribution trailers.
#
# Usage: scripts/check-commit-msg.sh <commit-msg-file>
#
# Mirrors the repo commit conventions (see AGENTS.md): the subject is a
# Conventional Commit, and the message carries no automated-attribution lines
# (attribution is disabled for this project). Exits non-zero with a clear,
# quotable message on failure.
set -euo pipefail

msg_file="${1:-}"
if [ -z "$msg_file" ] || [ ! -f "$msg_file" ]; then
  echo "check-commit-msg: missing commit message file argument" >&2
  exit 1
fi

# Subject = first non-comment, non-empty line.
subject="$(grep -vE '^[[:space:]]*#' "$msg_file" | sed '/^[[:space:]]*$/d' | head -n 1)"

case "$subject" in
  "Merge "* | "Revert "* | "fixup! "* | "squash! "*)
    : # generated subjects are allowed through
    ;;
  *)
    if ! printf '%s' "$subject" | grep -qE '^(feat|fix|refactor|docs|test|chore|perf|ci)(\([a-z0-9._-]+\))?!?: .+'; then
      echo "check-commit-msg: subject must be a Conventional Commit:" >&2
      echo "  <type>: <description>   (types: feat fix refactor docs test chore perf ci)" >&2
      echo "got: ${subject:-<empty>}" >&2
      exit 1
    fi
    ;;
esac

# Reject automated-attribution lines (trailer-style, to avoid prose false-positives).
if grep -qiE '^[[:space:]]*co-authored-by:|generated with \[?claude' "$msg_file" \
  || grep -qF '🤖' "$msg_file"; then
  echo "check-commit-msg: remove automated-attribution lines" >&2
  echo "  (Co-authored-by: / 'Generated with Claude' / 🤖) — attribution is disabled for this repo." >&2
  exit 1
fi

exit 0
