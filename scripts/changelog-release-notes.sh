#!/usr/bin/env bash
# Extract the CHANGELOG.md section for a given release tag.
#
# Usage: scripts/changelog-release-notes.sh <tag> [changelog-path]
#
# Prints the body of the matching `## <tag>` section (heading line excluded,
# leading/trailing blank lines trimmed) to stdout. If no matching section is
# found, falls back to printing the tag name so a release always has notes.
#
# Matching is exact on the version token: a tag of `v0.1.0` matches a heading
# `## v0.1.0` or `## v0.1.0 - 2026-06-19`, but not `## v0.1.0-rc1`.
set -euo pipefail

tag="${1:-}"
changelog="${2:-CHANGELOG.md}"

if [ -z "$tag" ]; then
  echo "usage: $0 <tag> [changelog-path]" >&2
  exit 2
fi

notes=""
if [ -f "$changelog" ]; then
  notes="$(
    awk -v tag="$tag" '
      # Section heading: "## <token> ..." — capture when <token> == tag.
      /^## / {
        split($0, parts, " ")
        if (capture) { capture = 0 }
        if (parts[2] == tag) { capture = 1; next }
      }
      capture { print }
    ' "$changelog"
  )"
  # Trim leading and trailing blank lines.
  notes="$(printf '%s\n' "$notes" | sed -e '/./,$!d' | sed -e ':a' -e '/^\s*$/{$d;N;ba}')"
fi

if [ -z "$notes" ]; then
  printf '%s\n' "$tag"
else
  printf '%s\n' "$notes"
fi
