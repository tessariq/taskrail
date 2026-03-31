#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

rm -rf .agents/skills .claude/skills
mkdir -p .agents/skills .claude/skills
cp -R skills/. .agents/skills/
cp -R skills/. .claude/skills/

printf 'mirrored canonical skills into .agents/skills and .claude/skills\n'
