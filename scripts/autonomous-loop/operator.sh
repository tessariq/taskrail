#!/usr/bin/env bash
# Temporary, repository-local parent-agent bridge. The runner remains the sole
# authority for task selection, lifecycle, integration, Git, and delivery.
set -uo pipefail

ROOT="${AUTONOMOUS_LOOP_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)}"
LOOP_DIR="$ROOT/scripts/autonomous-loop"
RUNNER="$LOOP_DIR/run.sh"
QUEUE_PATH=$LOOP_DIR/queue.tsv
RUNS_DIR="$ROOT/planning/artifacts/runs"
REMOTE_CI_STATUS=not_checked
RECOVERY_STATUS=unchecked
OBSERVED_INTERRUPT=""
DIAGNOSTIC_PRESERVATION=pass
LAUNCHED_COUNT=0
LAUNCHED_IDS=""

operator_log() {
  printf '[loop-operator] %s\n' "$*"
}

operator_error() {
  printf '[loop-operator] ERROR: %s\n' "$*" >&2
}

ask() {
  local prompt="$1" default="${2:-}" required="${3:-0}"
  while true; do
    if [[ -n "$default" ]]; then
      printf '%s [%s]: ' "$prompt" "$default" >&2
    else
      printf '%s: ' "$prompt" >&2
    fi
    if ! IFS= read -r ANSWER; then
      operator_error "input ended before all choices were explicit"
      return 1
    fi
    ANSWER="${ANSWER:-$default}"
    [[ $required -eq 0 || -n "$ANSWER" ]] && return 0
    operator_error "a value is required"
  done
}

positive_integer() {
  [[ "$1" =~ ^[1-9][0-9]*$ ]]
}

positive_duration() {
  [[ "$1" =~ ^[1-9][0-9]*(s|m|h)$ ]]
}

print_command() {
  printf '[loop-operator]'
  printf ' %q' "$@"
  printf '\n'
}

remote_main() {
  local output
  output="$(git -C "$ROOT" ls-remote --refs origin refs/heads/main 2>/dev/null)" || return 1
  [[ -n "$output" && "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" == "1" ]] || return 1
  printf '%s\n' "${output%%[[:space:]]*}"
}

check_tools_and_binary() {
  local backend="$1" tool
  for tool in bash git task taskrail gh go setsid mkfifo tee sha256sum stat; do
    command -v "$tool" >/dev/null 2>&1 || {
      operator_error "required tool is unavailable: $tool"
      return 1
    }
  done
  command -v "$backend" >/dev/null 2>&1 || {
    operator_error "selected backend CLI is unavailable: $backend"
    return 1
  }
  [[ -x "$RUNNER" ]] || {
    operator_error "runner is missing or not executable: $RUNNER"
    return 1
  }
  (cd "$ROOT" && task taskrail:check) || {
    operator_error "Taskrail binary freshness check failed; apply its named remedy"
    return 1
  }
}

workflow_requested() {
  local workflow="$1" requested_csv="$2" requested
  IFS=',' read -r -a requested_workflows <<<"$requested_csv"
  for requested in "${requested_workflows[@]}"; do
    requested="${requested#${requested%%[![:space:]]*}}"
    requested="${requested%${requested##*[![:space:]]}}"
    [[ "$workflow" == "$requested" ]] && return 0
  done
  return 1
}

read_ci_rows() {
  local head="$1"
  gh run list --commit "$head" --limit 1000 \
    --json databaseId,workflowName,headSha,status,conclusion \
    --jq '.[] | [.workflowName,.headSha,.status,(.conclusion // ""),(.databaseId|tostring)] | @tsv'
}

classify_ci_rows() {
  local head="$1" requested_csv="$2" rows="$3"
  local workflow row_head status conclusion database_id requested missing=0 pending=0 failed=0 cancelled=0 found=0
  local -A seen=()
  while IFS=$'\t' read -r workflow row_head status conclusion database_id; do
    [[ -n "$workflow" && "$row_head" == "$head" ]] || continue
    found=$((found + 1))
    seen["$workflow"]=1
    operator_log "remote workflow: $workflow id=$database_id status=$status conclusion=${conclusion:-none} head=$row_head"
    if [[ "$status" != "completed" ]]; then
      pending=1
    elif [[ "$conclusion" == "cancelled" ]]; then
      cancelled=1
    elif [[ "$conclusion" != "success" ]]; then
      failed=1
    fi
  done <<<"$rows"

  IFS=',' read -r -a requested_workflows <<<"$requested_csv"
  for requested in "${requested_workflows[@]}"; do
    requested="${requested#${requested%%[![:space:]]*}}"
    requested="${requested%${requested##*[![:space:]]}}"
    [[ -n "$requested" ]] || continue
    if [[ -z "${seen[$requested]+x}" ]]; then
      operator_log "remote workflow: $requested status=missing head=$head"
      missing=1
    fi
  done
  ((found > 0)) || missing=1

  if ((failed == 1)); then
    CI_CLASSIFICATION=fail
  elif ((cancelled == 1)); then
    CI_CLASSIFICATION=cancelled
  elif ((pending == 1)); then
    CI_CLASSIFICATION=pending
  elif ((missing == 1)); then
    CI_CLASSIFICATION=missing
  else
    CI_CLASSIFICATION=pass
  fi
}

observe_remote_ci() {
  local head="$1" requested_csv="$2" timeout_seconds="$3" poll_seconds="$4"
  local started now rows digest last_success_digest="" stable_success=0
  REMOTE_CI_STATUS=missing
  started="$(date +%s)"
  while true; do
    if ! rows="$(read_ci_rows "$head" 2>&1)"; then
      operator_error "remote CI observation failed: $rows"
      REMOTE_CI_STATUS=missing
      return 1
    fi
    classify_ci_rows "$head" "$requested_csv" "$rows"
    REMOTE_CI_STATUS="$CI_CLASSIFICATION"
    case "$REMOTE_CI_STATUS" in
      pass)
        digest="$(printf '%s\n' "$rows" | sort | sha256sum | awk '{print $1}')"
        if [[ "$digest" == "$last_success_digest" ]]; then
          stable_success=$((stable_success + 1))
        else
          last_success_digest="$digest"
          stable_success=1
        fi
        if ((stable_success >= 2)); then
          operator_log "remote workflow discovery is stable across two polls"
          return 0
        fi
        ;;
      fail|cancelled) return 0 ;;
      *) stable_success=0; last_success_digest="" ;;
    esac
    now="$(date +%s)"
    if ((now - started >= timeout_seconds)); then
      [[ "$REMOTE_CI_STATUS" != pass ]] || REMOTE_CI_STATUS=pending
      return 0
    fi
    sleep "$poll_seconds"
  done
}

verify_published_identity() {
  local expected="$1" head branch_head remote
  head="$(git -C "$ROOT" rev-parse HEAD)" || return 1
  branch_head="$(git -C "$ROOT" rev-parse refs/heads/main)" || return 1
  remote="$(remote_main)" || return 1
  [[ "$head" == "$expected" && "$branch_head" == "$expected" && "$remote" == "$expected" ]]
}

runner_field() {
  local file="$1" label="$2"
  sed -n "s/^\[autonomous-loop\] $label//p" "$file" | tail -n 1
}

summarize_runner() {
  local log_file="$1" id line index=0 outcome
  local -a ranked=()
  local -A seen=() worker_outcome=()

  LAUNCHED_COUNT=0
  LAUNCHED_IDS=""

  while IFS= read -r line; do
    id="${line#*worker launched: }"
    id="${id%% (*}"
    [[ -n "$id" && -z "${seen[$id]+x}" ]] || continue
    ranked+=("$id")
    seen["$id"]=1
  done < <(grep -E '^\[autonomous-loop\] worker launched: ' "$log_file" 2>/dev/null || true)
  while IFS= read -r line; do
    id="${line#*worker done: }"
    outcome="${id#* (}"
    outcome="${outcome%)}"
    id="${id%% (*}"
    worker_outcome["$id"]="$outcome"
  done < <(grep -E '^\[autonomous-loop\] worker done: ' "$log_file" 2>/dev/null || true)
  while IFS= read -r line; do
    id="${line#*worker failed: }"
    id="${id%% (*}"
    worker_outcome["$id"]="failed"
  done < <(grep -E '^\[autonomous-loop\] worker failed: ' "$log_file" 2>/dev/null || true)

  if ((${#ranked[@]} == 0)); then
    id="$(sed -n 's/^\[autonomous-loop\] starting \([^ ]*\).*/\1/p' "$log_file" | tail -n 1)"
    [[ -z "$id" ]] || ranked+=("$id")
  fi
  LAUNCHED_COUNT=${#ranked[@]}
  if ((${#ranked[@]} > 0)); then
    LAUNCHED_IDS="$(IFS=,; printf '%s' "${ranked[*]}")"
  fi
  for id in "${ranked[@]}"; do
    index=$((index + 1))
    operator_log "worker[$index]: $id - ${worker_outcome[$id]:-terminal result unavailable}"
  done
  ((${#ranked[@]} > 0)) || operator_log "worker: no launched worker was reported"

  while IFS= read -r line; do
    operator_log "integration: ${line#*integrated: } - integrated"
  done < <(grep -E '^\[autonomous-loop\] integrated: ' "$log_file" 2>/dev/null | awk '!seen[$0]++' || true)
  while IFS= read -r line; do
    operator_log "integration: ${line#*unpublished: } - unpublished"
  done < <(grep -E '^\[autonomous-loop\] unpublished: ' "$log_file" 2>/dev/null | awk '!seen[$0]++' || true)
  while IFS= read -r line; do
    operator_log "integration-conflict: ${line#*integration child start: } - child started"
  done < <(grep -E '^\[autonomous-loop\] integration child start: ' "$log_file" 2>/dev/null || true)
  while IFS= read -r line; do
    operator_log "integration-conflict: ${line#*integration child done: } - child completed"
  done < <(grep -E '^\[autonomous-loop\] integration child done: ' "$log_file" 2>/dev/null || true)

  if grep -Fq '[autonomous-loop] local aggregate gate: pass' "$log_file"; then
    LOCAL_GATE_STATUS=pass
  elif grep -Fq 'batch gate failed' "$log_file"; then
    LOCAL_GATE_STATUS=fail
  else
    LOCAL_GATE_STATUS=not_reported
  fi
  operator_log "local-gate: $LOCAL_GATE_STATUS"
}

handle_supervisor_signal() {
  OBSERVED_INTERRUPT="$1"
  operator_log "operator received $1; the existing runner and launched siblings will be allowed to settle"
}

run_supervised() {
  local log_file="$1"; shift
  local fifo="$log_file.fifo" runner_pid tee_pid rc tee_rc
  rm -f "$fifo"
  mkfifo "$fifo" || return 1
  "${AUTONOMOUS_OPERATOR_TEE:-tee}" -a "$log_file" <"$fifo" &
  tee_pid=$!
  setsid "$@" >"$fifo" 2>&1 &
  runner_pid=$!
  trap 'handle_supervisor_signal INT' INT
  trap 'handle_supervisor_signal TERM' TERM
  trap 'handle_supervisor_signal HUP' HUP
  while true; do
    wait "$runner_pid"
    rc=$?
    kill -0 "$runner_pid" 2>/dev/null || break
  done
  wait "$tee_pid" 2>/dev/null
  tee_rc=$?
  if ((tee_rc != 0)); then
    DIAGNOSTIC_PRESERVATION=fail
    operator_error "operator stream preservation failed with exit $tee_rc"
  fi
  rm -f "$fifo"
  trap - INT TERM HUP
  return "$rc"
}

bundle_value() {
  local bundle="$1" name="$2"
  [[ -f "$bundle/$name" && ! -L "$bundle/$name" ]] || return 1
  IFS= read -r BUNDLE_VALUE <"$bundle/$name" || true
}

check_manifest() {
  local manifest="$1" kind="$2" skip_id="${3:-}" line path digest
  while IFS=$'\t' read -r path digest; do
    [[ -n "$path" && "$digest" =~ ^[0-9a-f]{64}$ ]] || return 1
    if [[ "$kind" == task ]]; then
      [[ "$path" == planning/tasks/*.md && "$path" != *..* ]] || return 1
      [[ "$path" != "planning/tasks/$skip_id.md" ]] || continue
      path="$ROOT/$path"
    else
      [[ "$path" == "$ROOT"/planning/artifacts/verify/*/report.json && "$path" != *..* ]] || return 1
    fi
    [[ -f "$path" && ! -L "$path" && "$(sha256sum "$path" | awk '{print $1}')" == "$digest" ]] || return 1
  done <"$manifest"
}

validate_xdg_path() {
  local state_home="$1" path="" part
  IFS='/' read -r -a path_parts <<<"${state_home#/}/taskrail/autonomous-loop"
  for part in "${path_parts[@]}"; do
    [[ -n "$part" ]] || continue
    path="$path/$part"
    [[ -d "$path" && ! -L "$path" ]] || return 1
  done
}

candidate_matches_worktree() {
  local candidate="$1" record mode expected_hash path actual_hash actual_mode
  local -A candidate_paths=() working_paths=()
  while IFS= read -r -d '' record; do
    mode="${record%%$'\t'*}"
    record="${record#*$'\t'}"
    expected_hash="${record%%$'\t'*}"
    path="${record#*$'\t'}"
    [[ -n "$path" && -z "${candidate_paths[$path]+x}" ]] || return 1
    candidate_paths["$path"]=1
    [[ -f "$ROOT/$path" || -L "$ROOT/$path" ]] || return 1
    actual_hash="$(git -C "$ROOT" hash-object -- "$path")" || return 1
    [[ "$actual_hash" == "$expected_hash" ]] || return 1
    if [[ -L "$ROOT/$path" ]]; then
      actual_mode=120000
    elif [[ -x "$ROOT/$path" ]]; then
      actual_mode=100755
    else
      actual_mode=100644
    fi
    [[ "$actual_mode" == "$mode" ]] || return 1
  done < <(git -C "$ROOT" ls-tree -rz --format='%(objectmode)%x09%(objectname)%x09%(path)' "$candidate")
  ((${#candidate_paths[@]} > 0)) || return 1
  while IFS= read -r -d '' path; do
    [[ -f "$ROOT/$path" || -L "$ROOT/$path" ]] || continue
    working_paths["$path"]=1
  done < <(git -C "$ROOT" ls-files -co --exclude-standard -z)
  [[ ${#working_paths[@]} -eq ${#candidate_paths[@]} ]] || return 1
  for path in "${!working_paths[@]}"; do
    [[ -n "${candidate_paths[$path]+x}" ]] || return 1
  done
}

inspect_recovery_bundle() {
  local bundle="$1" state_home expected_root repo id outcome base_head base_remote base_index candidate report report_hash message_hash entry name base
  local report_result report_fields generated verification_id expected_generated task_status run_log delivered delivered_message expected_message verification key subject
  local -a bundle_entries=()
  local -A allowed=(
    [schema_version]=1 [repository]=1 [task_id]=1 [outcome]=1 [base_head]=1
    [base_remote]=1 [base_index]=1 [candidate_tree]=1 [report_path]=1
    [report_sha256]=1 [generated_at]=1 [queue_sha256]=1 [commit-message]=1
    [commit_message_sha256]=1 [existing-reports]=1 [task-manifest]=1 [run_log]=1
    [timeout]=1 [created_at]=1 [COMPLETE]=1 [DELIVERED]=1
  )
  RECOVERY_STATUS=invalid
  [[ "$bundle" == /* && -d "$bundle" && ! -L "$bundle" ]] || {
    operator_error "recovery path is not an absolute, real directory"
    return 1
  }
  state_home="${XDG_STATE_HOME:-${HOME:+$HOME/.local/state}}"
  [[ -n "$state_home" && "$state_home" == /* ]] || return 1
  validate_xdg_path "$state_home" || {
    operator_error "recovery path contains a missing or linked XDG component"
    return 1
  }
  expected_root="$state_home/taskrail/autonomous-loop/$(printf '%s' "$ROOT" | sha256sum | awk '{print $1}')"
  [[ "$(dirname "$bundle")" == "$expected_root" ]] || {
    operator_error "bundle is not beneath this repository's private XDG state root"
    return 1
  }
  [[ "$(stat -c '%u:%a' "$expected_root")" == "$(id -u):700" && "$(stat -c '%u:%a' "$bundle")" == "$(id -u):700" ]] || {
    operator_error "recovery storage ownership or permissions are unsafe"
    return 1
  }
  shopt -s dotglob nullglob
  bundle_entries=("$bundle"/*)
  shopt -u dotglob nullglob
  for entry in "${bundle_entries[@]}"; do
    base="${entry##*/}"
    [[ -n "${allowed[$base]+x}" && -f "$entry" && ! -L "$entry" ]] || {
      operator_error "recovery bundle contains an unsafe or unexpected entry: $base"
      return 1
    }
    [[ "$(stat -c '%u:%a' "$entry")" == "$(id -u):600" ]] || return 1
  done
  for name in schema_version repository task_id outcome base_head base_remote base_index candidate_tree report_path report_sha256 generated_at queue_sha256 commit-message commit_message_sha256 existing-reports task-manifest run_log timeout created_at COMPLETE; do
    bundle_value "$bundle" "$name" || {
      operator_error "incomplete recovery bundle: missing $name"
      return 1
    }
  done
  bundle_value "$bundle" COMPLETE; [[ "$BUNDLE_VALUE" == complete ]] || return 1
  bundle_value "$bundle" schema_version; [[ "$BUNDLE_VALUE" == 1 ]] || return 1
  bundle_value "$bundle" repository; repo="$BUNDLE_VALUE"
  [[ "$repo" == "$ROOT" ]] || {
    operator_error "recovery bundle belongs to another repository"
    return 1
  }
  bundle_value "$bundle" task_id; id="$BUNDLE_VALUE"
  [[ -f "$ROOT/planning/tasks/$id.md" ]] || return 1
  bundle_value "$bundle" outcome; outcome="$BUNDLE_VALUE"
  task_status="$(awk '$1 == "status:" { print $2; exit }' "$ROOT/planning/tasks/$id.md")"
  case "$outcome:$task_status" in
    completed_pass:completed) report_result=pass ;;
    blocked_fail:blocked|rework_fail:in_progress) report_result=fail ;;
    *) return 1 ;;
  esac
  bundle_value "$bundle" base_head; base_head="$BUNDLE_VALUE"
  bundle_value "$bundle" base_remote; base_remote="$BUNDLE_VALUE"
  bundle_value "$bundle" base_index; base_index="$BUNDLE_VALUE"
  bundle_value "$bundle" candidate_tree; candidate="$BUNDLE_VALUE"
  for entry in "$base_head" "$base_remote" "$base_index" "$candidate"; do
    [[ "$entry" =~ ^[0-9a-f]{40}$ ]] || return 1
  done
  bundle_value "$bundle" report_path; report="$BUNDLE_VALUE"
  [[ "$report" != /* && "$report" != *..* && -f "$ROOT/$report" ]] || return 1
  bundle_value "$bundle" report_sha256; report_hash="$BUNDLE_VALUE"
  [[ "$(sha256sum "$ROOT/$report" | awk '{print $1}')" == "$report_hash" ]] || return 1
  report_fields="$(go run "$LOOP_DIR/check-report.go" "$ROOT/$report" "$id" "$report_result" 2>/dev/null)" || return 1
  generated="${report_fields%%$'\n'*}"
  verification_id="$(printf '%s\n' "$report_fields" | sed -n '4p')"
  bundle_value "$bundle" generated_at; expected_generated="$BUNDLE_VALUE"
  [[ "$generated" == "$expected_generated" ]] || return 1
  verification="$(awk '$1 == "last_verification_result:" { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }' "$ROOT/planning/STATE.md")"
  expected_message="$report_result for $id at $generated"
  [[ -z "$verification_id" ]] || expected_message+=" id $verification_id"
  [[ "$verification" == "$expected_message" ]] || return 1
  bundle_value "$bundle" commit_message_sha256; message_hash="$BUNDLE_VALUE"
  [[ -f "$bundle/commit-message" && "$(sha256sum "$bundle/commit-message" | awk '{print $1}')" == "$message_hash" ]] || return 1
  "$ROOT/scripts/check-commit-msg.sh" "$bundle/commit-message" >/dev/null || return 1
  [[ "$id" =~ ^(T-[0-9]+)(-|$) ]] || return 1
  key="${BASH_REMATCH[1]}"
  subject="$(grep -vE '^[[:space:]]*#' "$bundle/commit-message" | sed '/^[[:space:]]*$/d' | head -n 1)"
  [[ "$subject" =~ \($key\)$ ]] || return 1
  bundle_value "$bundle" queue_sha256
  [[ "$(sha256sum "$QUEUE_PATH" | awk '{print $1}')" == "$BUNDLE_VALUE" ]] || return 1
  check_manifest "$bundle/existing-reports" report || return 1
  check_manifest "$bundle/task-manifest" task "$id" || return 1
  bundle_value "$bundle" run_log; run_log="$BUNDLE_VALUE"
  [[ "$run_log" == planning/artifacts/runs/* && "$run_log" != *..* && -f "$ROOT/$run_log" ]] || return 1
  [[ "$(git -C "$ROOT" symbolic-ref --quiet --short HEAD 2>/dev/null)" == main ]] || return 1
  candidate_matches_worktree "$candidate" || {
    operator_error "current source bytes do not match the recovery candidate identity"
    return 1
  }
  operator_log "recovery identity: repository=$repo task=$id outcome=$outcome"
  operator_log "recovery identity: base=$base_head report=$report message-sha256=$message_hash candidate=$candidate"
  if [[ -e "$bundle/DELIVERED" || -L "$bundle/DELIVERED" ]]; then
    [[ -f "$bundle/DELIVERED" && ! -L "$bundle/DELIVERED" ]] || return 1
    bundle_value "$bundle" DELIVERED; delivered="$BUNDLE_VALUE"
    [[ "$delivered" =~ ^[0-9a-f]{40}$ ]] || return 1
    [[ "$(git -C "$ROOT" rev-parse HEAD)" == "$delivered" && \
      "$(git -C "$ROOT" rev-parse refs/heads/main)" == "$delivered" && \
      "$(remote_main)" == "$delivered" ]] || return 1
    [[ "$(git -C "$ROOT" rev-parse "$delivered^")" == "$base_head" && \
      "$(git -C "$ROOT" rev-parse "$delivered^{tree}")" == "$candidate" && \
      "$(git -C "$ROOT" write-tree)" == "$candidate" ]] || return 1
    delivered_message="$(git -C "$ROOT" show -s --format=%B "$delivered")"
    expected_message="$(<"$bundle/commit-message")"
    [[ "$delivered_message" == "$expected_message" ]] || return 1
    [[ -z "$(git -C "$ROOT" status --porcelain=v1 --untracked-files=all --ignore-submodules=none)" ]] || return 1
    RECOVERY_STATUS=delivered
    operator_log "current source preconditions: delivered commit/tree/message and local main/HEAD/origin agree"
    operator_log "recovery bundle is already delivered; delivery resume is forbidden"
    return 0
  fi
  [[ "$(git -C "$ROOT" rev-parse HEAD)" == "$base_head" ]] || {
    operator_error "recovery base HEAD is stale"
    return 1
  }
  [[ "$(remote_main)" == "$base_remote" ]] || {
    operator_error "recovery base remote is stale"
    return 1
  }
  [[ "$(git -C "$ROOT" write-tree)" == "$base_index" ]] || {
    operator_error "recovery index differs from the bundle base"
    return 1
  }
  operator_log "current source preconditions: branch=main and exact HEAD/base/index/remote/candidate bytes match"
  RECOVERY_STATUS=undelivered
  return 0
}

preserve_diagnostic_path() {
  local path="$1"
  if [[ -e "$path" ]]; then
    operator_log "transient diagnostic retained: $path"
  else
    DIAGNOSTIC_PRESERVATION=fail
    operator_error "diagnostic preservation failed or path is unavailable: $path"
  fi
}

summarize_transient_diagnostics() {
  local log_file="$1" line path
  while IFS= read -r line; do
    path="${line#*: }"
    path="${path%% (*}"
    [[ "$path" == /* ]] || path="$ROOT/$path"
    preserve_diagnostic_path "$path"
  done < <(grep -E '^\[autonomous-loop\] retained (failed workspace|integration workspace|workspace root): ' "$log_file" 2>/dev/null || true)
}

main() {
  local backend model effort budget parallel timeout clone_depth retention workflows ci_timeout poll_seconds confirmation
  local stamp log_file dry_log confirm_log dry_rc live_rc pushed_head bundle quota source reset resume_confirmation dry_digest confirm_digest
  local -a args=() dry_command=() live_command=()

  cd "$ROOT" || { operator_error "cannot enter repository root"; return 2; }
  operator_log "Temporary source-checkout bridge; parent-agent operated and removed in full by T-258."
  operator_log "Provider-specific behavior is limited to the caller's backend, model, and effort choices."
  ask "Backend (claude or opencode)" claude || return 2; backend="$ANSWER"
  [[ "$backend" == claude || "$backend" == opencode ]] || { operator_error "invalid backend"; return 2; }
  ask "Model (blank keeps backend default)" "" || return 2; model="$ANSWER"
  ask "Effort/variant (blank keeps backend default)" "" || return 2; effort="$ANSWER"
  ask "Finite iteration budget" "" 1 || return 2; budget="$ANSWER"
  positive_integer "$budget" || { operator_error "iteration budget must be a positive integer"; return 2; }
  ask "Parallel width" 1 || return 2; parallel="$ANSWER"
  positive_integer "$parallel" || { operator_error "parallel width must be a positive integer"; return 2; }
  ask "Worker timeout" 2h || return 2; timeout="$ANSWER"
  positive_duration "$timeout" || { operator_error "timeout must be a positive s, m, or h duration"; return 2; }
  clone_depth=""
  retention=""
  if ((parallel > 1 && budget > 1)); then
    ask "Clone depth (positive integer or full)" 1 || return 2; clone_depth="$ANSWER"
    [[ "$clone_depth" == full ]] || positive_integer "$clone_depth" || { operator_error "invalid clone depth"; return 2; }
    ask "Workspace retention (never, failure, or always)" failure || return 2; retention="$ANSWER"
    case "$retention" in never|failure|always) ;; *) operator_error "invalid retention policy"; return 2 ;; esac
  fi
  ask "Required exact workflow names, comma-separated" "CI,Planning checks,CodeQL" || return 2; workflows="$ANSWER"
  [[ -n "$workflows" ]] || { operator_error "at least one workflow name is required"; return 2; }
  ask "Remote CI discovery/wait timeout in seconds" 300 || return 2; ci_timeout="$ANSWER"
  positive_integer "$ci_timeout" || { operator_error "CI timeout must be a positive integer"; return 2; }
  ask "Remote CI polling interval in seconds" 5 || return 2; poll_seconds="$ANSWER"
  [[ "$poll_seconds" =~ ^[0-9]+$ ]] || { operator_error "CI polling interval must be a non-negative integer"; return 2; }

  args=(--backend "$backend" --timeout "$timeout" --max-iterations "$budget" --parallel "$parallel")
  [[ -z "$model" ]] || args+=(--model "$model")
  [[ -z "$effort" ]] || args+=(--effort "$effort")
  [[ -z "$clone_depth" ]] || args+=(--clone-depth "$clone_depth")
  [[ -z "$retention" ]] || args+=(--keep-workspaces "$retention")
  dry_command=("$RUNNER" "${args[@]}" --dry-run)
  live_command=("$RUNNER" "${args[@]}")

  check_tools_and_binary "$backend" || return 2
  operator_log "Exact dry-run command:"
  print_command "${dry_command[@]}"
  dry_log="$(mktemp)" || return 2
  "${dry_command[@]}" 2>&1 | tee "$dry_log"
  dry_rc=${PIPESTATUS[0]}
  ((dry_rc == 0)) || { operator_error "dry-run refused with exit $dry_rc"; rm -f "$dry_log"; return "$dry_rc"; }
  operator_log "The dry-run above is the exact frontier and exclusions at its frozen base."
  operator_log "Integration is ranked serial replay; delivery is a guarded non-force fast-forward and push."
  operator_log "The runner alone owns worker lifecycle, integration, Git, planning, and delivery mutations."
  operator_log "Remote CI is separate external evidence and will be checked only for the exact pushed head."
  ask "Type RUN to confirm this exact one-shot live invocation" "" 1 || return 2; confirmation="$ANSWER"
  [[ "$confirmation" == RUN ]] || { operator_error "confirmation refused; no live runner was invoked"; rm -f "$dry_log"; return 20; }

  check_tools_and_binary "$backend" || return 2
  confirm_log="$(mktemp)" || return 2
  "${dry_command[@]}" >"$confirm_log" 2>&1
  dry_rc=$?
  ((dry_rc == 0)) || { operator_error "confirmed dry-run no longer passes; start a new invocation"; rm -f "$dry_log" "$confirm_log"; return "$dry_rc"; }
  dry_digest="$(sha256sum "$dry_log" | awk '{print $1}')"
  confirm_digest="$(sha256sum "$confirm_log" | awk '{print $1}')"
  if [[ "$dry_digest" != "$confirm_digest" ]]; then
    operator_error "dry-run changed after confirmation; review the new plan in a fresh invocation"
    rm -f "$dry_log" "$confirm_log"
    return 20
  fi
  operator_log "confirmed dry-run snapshot recheck: unchanged"
  rm -f "$dry_log" "$confirm_log"
  # Keep the source-checkout guard immediately adjacent to the mutating runner.
  check_tools_and_binary "$backend" || return 2
  mkdir -p "$RUNS_DIR" || { operator_error "cannot preserve diagnostics under ignored artifacts"; return 2; }
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  log_file="$RUNS_DIR/$stamp-operator-$$.log"
  : >"$log_file" || { operator_error "cannot create operator diagnostic log"; return 2; }
  operator_log "Live command (invoked once):"
  print_command "${live_command[@]}"
  run_supervised "$log_file" "${live_command[@]}"
  live_rc=$?
  preserve_diagnostic_path "$log_file"
  [[ -d "$RUNS_DIR" ]] && operator_log "available coordinator and worker diagnostics remain under: $RUNS_DIR" || \
    operator_error "coordinator/worker diagnostic preservation directory is unavailable: $RUNS_DIR"
  operator_log "Provider and wrapper streams may be sensitive, incomplete, and are not Taskrail evidence."
  summarize_transient_diagnostics "$log_file"
  [[ -z "$OBSERVED_INTERRUPT" ]] || operator_log "supervision continued after operator signal: $OBSERVED_INTERRUPT"
  summarize_runner "$log_file"

  pushed_head="$(sed -n -E 's/^\[autonomous-loop\] (published batch head: |completed and pushed: [^ ]+ \(|resumed and pushed: [^ ]+ \()([0-9a-f]{40})\)?$/\2/p' "$log_file" | tail -n 1)"
  if [[ -n "$pushed_head" ]]; then
    if verify_published_identity "$pushed_head"; then
      operator_log "delivery: pass (local main, HEAD, and origin/main agree at $pushed_head)"
      observe_remote_ci "$pushed_head" "$workflows" "$ci_timeout" "$poll_seconds" || true
    else
      operator_log "delivery: fail (published identity does not match local main, HEAD, and origin/main)"
      REMOTE_CI_STATUS=not_checked
    fi
  else
    operator_log "delivery: not_published"
    REMOTE_CI_STATUS=not_checked
  fi
  operator_log "remote-CI: $REMOTE_CI_STATUS"

  bundle="$(sed -n 's/^\[autonomous-loop\] recovery bundle: //p' "$log_file" | tail -n 1)"
  if [[ -n "$bundle" ]]; then
    operator_log "runner-reported recovery bundle: $bundle"
    if inspect_recovery_bundle "$bundle"; then
      if [[ "$RECOVERY_STATUS" == undelivered ]]; then
        operator_log "This bundle can resume parent-owned delivery only; it cannot resume or launch an agent."
        operator_log "Exact delivery-only command:"
        print_command "$RUNNER" --resume-delivery "$bundle"
        ask "Type RESUME-DELIVERY to invoke that exact command once" "" 1 || return 2
        resume_confirmation="$ANSWER"
        if [[ "$resume_confirmation" == RESUME-DELIVERY ]]; then
          check_tools_and_binary "$backend" || return 2
          run_supervised "$log_file" "$RUNNER" --resume-delivery "$bundle"
          live_rc=$?
          pushed_head="$(sed -n -E 's/^\[autonomous-loop\] resumed and pushed: [^ ]+ \(([0-9a-f]{40})\)$/\1/p' "$log_file" | tail -n 1)"
          if [[ -n "$pushed_head" ]] && verify_published_identity "$pushed_head" && \
            inspect_recovery_bundle "$bundle" && [[ "$RECOVERY_STATUS" == delivered ]]; then
            operator_log "delivery: pass (delivery-only recovery)"
            observe_remote_ci "$pushed_head" "$workflows" "$ci_timeout" "$poll_seconds" || true
            operator_log "remote-CI: $REMOTE_CI_STATUS"
          else
            operator_log "delivery: fail (recovery did not prove exact source/remote identity)"
          fi
        else
          operator_log "delivery recovery refused; it will not be retried"
        fi
      fi
    else
      operator_error "runner-reported bundle failed safe inspection; no recovery command was invoked"
      operator_log "safe next action: preserve the bundle and diagnostics for operator review; do not retry, replace, or mutate Git/planning state"
    fi
  elif ((live_rc != 0)); then
    operator_log "safe next action: inspect the retained diagnostics and ordinary terminal result; do not retry, replace a worker, or mutate the queue"
  fi

  ask "Did the selected backend or operator report quota exhaustion? (yes/no)" no || return 2; quota="$ANSWER"
  if [[ "$quota" == yes ]]; then
    ask "Source of the external quota observation" "" 1 || return 2; source="$ANSWER"
    ask "Quote the quota/reset observation exactly (include supplied timezone or offset)" "" 1 || return 2; reset="$ANSWER"
    operator_log "attributed external quota evidence from $source: $reset"
    operator_log "This is potentially heuristic external evidence, not a Taskrail outcome or attested reset instant."
    operator_log "Attempt accounting: configured cap iterations=$budget parallel=$parallel; launched=$LAUNCHED_COUNT ids=${LAUNCHED_IDS:-none}."
    operator_log "Every launched attempt remains consumed; unused capacity is not an attempt, refund, or carry-forward."
    operator_log "After any reported reset, start this bridge again in the foreground for a fresh preflight, exact dry-run, newly explicit finite budget, and fresh confirmation."
    operator_log "No background wake-up, persisted launch intent, worker/session resume, queue mutation, or automatic relaunch is scheduled."
  elif [[ "$quota" != no ]]; then
    operator_error "quota answer must be yes or no"
    return 2
  fi

  if [[ "$REMOTE_CI_STATUS" != pass && "$REMOTE_CI_STATUS" != not_checked ]]; then
    operator_log "safe next action: inspect exact-head workflow results; local gates or push success do not authorize a green claim or worker retry"
  fi
  if ((live_rc == 0)) && [[ "$REMOTE_CI_STATUS" == pass && -z "$OBSERVED_INTERRUPT" && "$DIAGNOSTIC_PRESERVATION" == pass ]]; then
    operator_log "overall: success"
    return 0
  fi
  operator_log "overall: non-success (runner=$live_rc remote-CI=$REMOTE_CI_STATUS interruption=${OBSERVED_INTERRUPT:-none} diagnostics=$DIAGNOSTIC_PRESERVATION)"
  return 1
}

if [[ "${AUTONOMOUS_LOOP_OPERATOR_LIBRARY:-0}" != 1 ]]; then
  main "$@"
fi
