#!/usr/bin/env bash
# Reject automated-attribution lines in a commit message.
#
# The repository message policy forbids agent attribution in history. It reaches
# a message as a trailer, as the bare agent link a wrapped trailer leaves on its
# own line, or as the robot mark. All three are rejected here: a trailer name
# that is not listed yet still carries its session link, which the URL patterns
# catch on whichever line it landed on.
#
# This is the single answer both the commit-msg hook and the pre-push scan read,
# so a message the commit hook would have refused cannot reach the remote
# through `--no-verify`, a rebase, or an amend.
set -euo pipefail

msg_file="${1:-}"
if [ -z "$msg_file" ] || [ ! -f "$msg_file" ]; then
  echo "check-attribution: missing commit message file argument" >&2
  exit 2
fi

if grep -qiE '^[[:space:]]*((co-authored-by|assisted-by|generated-by|(claude|amp|agent|codex|copilot)-(session|thread)(-id)?):|generated with[[:space:]])' "$msg_file" \
  || grep -qiE '(claude\.ai/code/session|ampcode\.com/threads|chatgpt\.com/(share|c)/|cursor\.com/agents)' "$msg_file" \
  || grep -qF '🤖' "$msg_file"; then
  exit 1
fi
