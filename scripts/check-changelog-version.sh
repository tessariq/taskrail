#!/usr/bin/env bash
# Pre-publish guard: fail unless CHANGELOG.md has a heading for the release tag.
#
# Usage: scripts/check-changelog-version.sh <tag> [changelog-path]
#
# Taskrail injects its version via -ldflags and has no in-code version constant,
# so the CHANGELOG `## v<version>` heading is the source of truth checked against
# the pushed tag. Exits non-zero when the heading is missing, blocking a release
# that would otherwise ship without documented notes.
set -euo pipefail

tag="${1:-}"
changelog="${2:-CHANGELOG.md}"

if [ -z "$tag" ]; then
  echo "usage: $0 <tag> [changelog-path]" >&2
  exit 2
fi

if [ ! -f "$changelog" ]; then
  echo "guard: $changelog not found" >&2
  exit 1
fi

found="$(
  awk -v tag="$tag" '
    /^## / {
      split($0, parts, " ")
      if (parts[2] == tag) { print "yes"; exit }
    }
  ' "$changelog"
)"

if [ "$found" != "yes" ]; then
  echo "guard: no '## $tag' heading found in $changelog" >&2
  echo "Add a '## $tag' section to $changelog before tagging $tag." >&2
  exit 1
fi

echo "guard: found '## $tag' heading in $changelog"
