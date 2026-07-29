#!/usr/bin/env bash
# Extract the CHANGELOG.md section for a given release tag.
#
# Usage: scripts/changelog-release-notes.sh <tag> [changelog-path]
#
# Prints the body of the matching `## <tag>` section (heading line excluded,
# leading/trailing blank lines trimmed) to stdout. Exits non-zero when the
# section is missing or empty so a release cannot publish manufactured notes.
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
        capture = (parts[2] == tag)
        next
      }
      capture { print }
    ' "$changelog"
  )"
  # Trim leading and trailing blank lines.
  notes="$(printf '%s\n' "$notes" | sed -e '/./,$!d' | sed -e ':a' -e '/^\s*$/{$d;N;ba}')"
fi

if [ -z "$notes" ]; then
  echo "guard: no non-empty '## $tag' section found in $changelog" >&2
  exit 1
fi

printf '%s\n' "$notes"
