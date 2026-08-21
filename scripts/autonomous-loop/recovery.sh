#!/usr/bin/env bash

recovery_root() {
  local state_home repo_hash
  state_home="${XDG_STATE_HOME:-${HOME:+$HOME/.local/state}}"
  [[ -n "$state_home" && "$state_home" == /* ]] || die "recovery requires absolute XDG_STATE_HOME or HOME" 2
  repo_hash="$(printf '%s' "$ROOT" | sha256sum | awk '{print $1}')"
  printf '%s/taskrail/autonomous-loop/%s\n' "$state_home" "$repo_hash"
}

ensure_recovery_root() {
  local state_home path part
  state_home="${XDG_STATE_HOME:-${HOME:+$HOME/.local/state}}"
  [[ -n "$state_home" && "$state_home" == /* ]] || die "recovery requires absolute XDG_STATE_HOME or HOME" 2
  path=""
  IFS='/' read -r -a parts <<<"${state_home#/}/taskrail/autonomous-loop"
  for part in "${parts[@]}"; do
    [[ -n "$part" ]] || continue
    path="$path/$part"
    [[ ! -L "$path" ]] || die "recovery path contains a symlink: $path" 2
    [[ -e "$path" ]] || mkdir "$path" || die "cannot create recovery directory: $path" 2
    [[ -d "$path" ]] || die "recovery path is not a directory: $path" 2
  done
}

bundle_write() {
  local dir="$1" name="$2" value="$3"
  printf '%s\n' "$value" >"$dir/$name"
  chmod 600 "$dir/$name"
}

create_recovery_bundle() {
  local id="$1" result="$2" report="$3" generated_at="$4"
  local before_head="$5" before_remote="$6" before_index="$7" root bundle tmp candidate
  root="$(recovery_root)"
  ensure_recovery_root
  mkdir -p "$root"
  [[ -d "$root" && ! -L "$root" ]] || die "$id recovery root is not a private directory" 2
  chmod 700 "$root"
  bundle="$root/$(date -u +%Y%m%dT%H%M%SZ)-$id-$$"
  tmp="$bundle.tmp"
  umask 077
  mkdir "$tmp" || die "$id cannot create recovery bundle" 2
  candidate="$(candidate_tree)" || die "$id cannot snapshot candidate tree"
  bundle_write "$tmp" schema_version 1
  bundle_write "$tmp" repository "$ROOT"
  bundle_write "$tmp" task_id "$id"
  bundle_write "$tmp" outcome "$result"
  bundle_write "$tmp" base_head "$before_head"
  bundle_write "$tmp" base_remote "$before_remote"
  bundle_write "$tmp" base_index "$before_index"
  bundle_write "$tmp" candidate_tree "$candidate"
  bundle_write "$tmp" report_path "${report#$ROOT/}"
  bundle_write "$tmp" report_sha256 "$(sha256sum "$report" | awk '{print $1}')"
  bundle_write "$tmp" generated_at "$generated_at"
  bundle_write "$tmp" queue_sha256 "$(sha256sum "$QUEUE" | awk '{print $1}')"
  cp "$COMMIT_MESSAGE" "$tmp/commit-message"
  chmod 600 "$tmp/commit-message"
  bundle_write "$tmp" commit_message_sha256 "$(sha256sum "$COMMIT_MESSAGE" | awk '{print $1}')"
  cp "$reports_before_manifest" "$tmp/existing-reports"
  chmod 600 "$tmp/existing-reports"
  cp "$before_manifest" "$tmp/task-manifest"
  chmod 600 "$tmp/task-manifest"
  bundle_write "$tmp" run_log "${RUN_LOG#$ROOT/}"
  bundle_write "$tmp" timeout "$TIMEOUT"
  bundle_write "$tmp" created_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  bundle_write "$tmp" COMPLETE complete
  chmod 700 "$tmp"
  mv "$tmp" "$bundle" || die "$id cannot publish recovery bundle"
  RECOVERY_BUNDLE="$bundle"
  runner_log "recovery bundle: $bundle"
}

bundle_read() {
  local bundle="$1" name="$2" file value
  file="$bundle/$name"
  [[ -f "$file" && ! -L "$file" ]] || die "incomplete recovery bundle: missing $name" 2
  IFS= read -r value <"$file" || true
  printf '%s\n' "$value"
}

validate_bundle_storage() {
  local bundle="$1" root entry base
  local -a entries=()
  root="$(recovery_root)"
  ensure_recovery_root
  [[ ! -L "$root" && ! -L "$bundle" ]] || die "recovery storage contains a symlink" 2
  [[ "$(dirname "$bundle")" == "$root" ]] || die "recovery bundle is outside the repository state root" 2
  [[ "$(stat -c '%u' "$root")" == "$(id -u)" && "$(stat -c '%a' "$root")" == "700" ]] || die "recovery root permissions are unsafe" 2
  [[ "$(stat -c '%u' "$bundle")" == "$(id -u)" && "$(stat -c '%a' "$bundle")" == "700" ]] || die "recovery bundle permissions are unsafe" 2
  shopt -s dotglob nullglob
  entries=("$bundle"/*)
  shopt -u dotglob nullglob
  for entry in "${entries[@]}"; do
    base="${entry##*/}"
    case "$base" in
      COMPLETE|base_head|base_index|base_remote|candidate_tree|commit-message|commit_message_sha256|created_at|existing-reports|generated_at|outcome|queue_sha256|report_path|report_sha256|repository|run_log|schema_version|task-manifest|task_id|timeout) ;;
      DELIVERED) die "recovery bundle was already delivered" 2 ;;
      *) die "recovery bundle contains unexpected entry: $base" 2 ;;
    esac
    [[ -f "$entry" && ! -L "$entry" && "$(stat -c '%u' "$entry")" == "$(id -u)" && "$(stat -c '%a' "$entry")" == "600" ]] || \
      die "recovery bundle entry is unsafe: $base" 2
  done
}

mark_delivered() {
  local bundle="$1" commit="$2" tmp
  [[ ! -e "$bundle/DELIVERED" && ! -L "$bundle/DELIVERED" ]] || die "recovery bundle was already delivered" 2
  tmp="$(mktemp "$bundle/.DELIVERED.XXXXXX")" || die "cannot create delivery marker"
  printf '%s\n' "$commit" >"$tmp"
  chmod 600 "$tmp"
  ln "$tmp" "$bundle/DELIVERED" || die "cannot mark recovery bundle delivered" 2
  rm -f "$tmp"
}

check_report() {
  local report="$1" id="$2" result="$3" output="$TMP_DIR/report-fields"
  if ! MISE_TRUSTED_CONFIG_PATHS="$ROOT/mise.toml" CGO_ENABLED=0 GOCACHE="$TMP_DIR/gocache" \
    mise exec -- go run "$LOOP_DIR/check-report.go" "$report" "$id" "$result" >"$output"; then
    return 1
  fi
  mapfile -t REPORT_FIELDS <"$output"
}

verification_summary() {
  local result="$1" id="$2" generated="$3" verification_id="${4:-}"
  printf '%s for %s at %s' "$result" "$id" "$generated"
  [[ -z "$verification_id" ]] || printf ' (verification_id=%s)' "$verification_id"
}

validate_report_binding() {
  local id="$1" result="$2" report="$3" expected_generated="$4" verification generated verification_id
  check_report "$report" "$id" "$result" || return 1
  generated="${REPORT_FIELDS[0]:-}"
  verification_id="${REPORT_FIELDS[3]:-}"
  [[ "$generated" == "$expected_generated" ]] || return 1
  verification="$(awk '$1 == "last_verification_result:" { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }' planning/STATE.md)"
  [[ "$verification" == "$(verification_summary "$result" "$id" "$generated" "$verification_id")" ]]
}

resume_delivery() {
  local bundle="$1" mode="${2:-resume}" id result before_head before_remote before_index expected_tree report generated
  local current_head current_remote subject key report_result committed_message expected_message
  [[ "$bundle" == /* && -d "$bundle" && ! -L "$bundle" ]] || die "invalid recovery bundle path" 2
  validate_bundle_storage "$bundle"
  [[ "$(bundle_read "$bundle" schema_version)" == "1" ]] || die "unsupported recovery bundle schema" 2
  [[ "$(bundle_read "$bundle" COMPLETE)" == "complete" ]] || die "incomplete recovery bundle" 2
  [[ "$(bundle_read "$bundle" repository)" == "$ROOT" ]] || die "recovery bundle belongs to another repository" 2
  id="$(bundle_read "$bundle" task_id)"
  result="$(bundle_read "$bundle" outcome)"
  before_head="$(bundle_read "$bundle" base_head)"
  before_remote="$(bundle_read "$bundle" base_remote)"
  before_index="$(bundle_read "$bundle" base_index)"
  expected_tree="$(bundle_read "$bundle" candidate_tree)"
  report="$ROOT/$(bundle_read "$bundle" report_path)"
  generated="$(bundle_read "$bundle" generated_at)"
  [[ "$(sha256sum "$bundle/commit-message" | awk '{print $1}')" == "$(bundle_read "$bundle" commit_message_sha256)" ]] || die "recovery bundle commit message was tampered with" 2
  [[ -f "$report" && "$(sha256sum "$report" | awk '{print $1}')" == "$(bundle_read "$bundle" report_sha256)" ]] || die "recovery report changed" 2
  assert_existing_reports_unchanged "$bundle/existing-reports" || die "existing verification evidence changed" 2
  assert_other_tasks_unchanged "$id" "$bundle/task-manifest"
  [[ "$(sha256sum "$QUEUE" | awk '{print $1}')" == "$(bundle_read "$bundle" queue_sha256)" ]] || die "recovery queue changed" 2
  [[ "$(git symbolic-ref --quiet --short HEAD 2>/dev/null)" == "main" ]] || die "recovery requires branch main" 2
  validate_queue
  assert_fresh_binary
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "recovery state is invalid" 2
  case "$result" in
    completed_pass) report_result=pass ;;
    blocked_fail | rework_fail) report_result=fail ;;
    *) die "recovery bundle has invalid outcome" 2 ;;
  esac
  validate_report_binding "$id" "$report_result" "$report" "$generated" || die "recovery lifecycle/report binding changed" 2
  "$ROOT/scripts/check-commit-msg.sh" "$bundle/commit-message" || die "recovery commit message is invalid" 2
  key="$(task_key "$id")"
  subject="$(grep -vE '^[[:space:]]*#' "$bundle/commit-message" | sed '/^[[:space:]]*$/d' | head -n 1)"
  [[ "$subject" =~ \($key\)$ ]] || die "recovery commit subject must end with ($key)" 2

  current_head="$(git rev-parse HEAD)"
  current_remote="$(remote_main)" || die "cannot resolve origin/main during recovery"
  if [[ "$current_head" == "$before_head" ]]; then
    [[ "$current_remote" == "$before_remote" ]] || die "recovery remote moved"
    [[ "$(git write-tree)" == "$before_index" ]] || die "recovery index changed"
    [[ "$(candidate_tree)" == "$expected_tree" ]] || die "recovery worktree changed"
    git add -A || die "recovery changes could not be staged"
    [[ "$(git write-tree)" == "$expected_tree" ]] || die "recovery staged tree mismatch"
    git commit -F "$bundle/commit-message" || die "recovery commit failed"
    current_head="$(git rev-parse HEAD)"
  fi
  [[ "$(git rev-parse HEAD^)" == "$before_head" ]] || die "recovery HEAD is not a direct child"
  [[ "$(git rev-list --count "$before_head..$current_head")" == "1" ]] || die "recovery produced more than one commit"
  [[ "$(git rev-parse HEAD^{tree})" == "$expected_tree" ]] || die "recovery commit tree changed"
  committed_message="$(git show -s --format=%B HEAD)"
  expected_message="$(<"$bundle/commit-message")"
  [[ "$committed_message" == "$expected_message" ]] || die "recovery commit message changed"
  [[ "$(git write-tree)" == "$expected_tree" ]] || die "recovery index changed before push"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all --ignore-submodules=none)" ]] || die "recovery worktree changed before push"
  [[ "$(candidate_tree)" == "$expected_tree" ]] || die "recovery candidate changed before push"
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "recovery pre-push validation failed"
  current_remote="$(remote_main)" || die "cannot resolve origin/main during recovery"
  if [[ "$current_remote" == "$before_remote" ]]; then
    [[ -z "$INTERRUPTED" ]] || die "recovery interrupted before push"
    git push origin "$current_head:main" || die "recovery push failed"
  elif [[ "$current_remote" != "$current_head" ]]; then
    die "recovery remote has an unexpected commit"
  fi
  [[ "$(remote_main)" == "$current_head" ]] || die "recovery push did not publish the expected commit"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all --ignore-submodules=none)" ]] || die "recovery delivery left a dirty tree"
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "recovery post-push validation failed"
  mark_delivered "$bundle" "$(git rev-parse HEAD)"
  if [[ "$mode" == "normal" ]]; then
    runner_log "completed and pushed: $id ($(git rev-parse HEAD))"
  else
    runner_log "resumed and pushed: $id ($(git rev-parse HEAD))"
    [[ "$result" != "blocked_fail" && "$result" != "rework_fail" ]] || return 1
  fi
}
