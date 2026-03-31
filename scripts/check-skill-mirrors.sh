#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

diff -ru --exclude='.DS_Store' skills .agents/skills
diff -ru --exclude='.DS_Store' skills .claude/skills

printf 'skill mirrors are in sync\n'
