#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOOP_DIR="$ROOT/scripts/autonomous-loop"
QUEUE="$LOOP_DIR/queue.tsv"
PROMPT="$LOOP_DIR/prompt.md"
RUNS_DIR="$ROOT/planning/artifacts/runs"
MAX_ITERATIONS=1
DRY_RUN=0
CHECK_QUEUE=0
TIMEOUT=2h
TIMEOUT_SECONDS=7200
KILL_GRACE_SECONDS=10
RESUME_BUNDLE=""
BACKEND=claude
BACKEND_SET=0
MODEL=""
MODEL_SET=0
EFFORT=""
EFFORT_SET=0
PARALLEL=1
PARALLEL_SET=0
CLONE_DEPTH=1
CLONE_DEPTH_SET=0
KEEP_WORKSPACES=failure
KEEP_WORKSPACES_SET=0
TMP_DIR=""
CHILD_DIR=""
LOCK_DIR=""
LOCK_TOKEN=""
ACTIVE_PGID=""
ACTIVE_WATCHDOG=""
RUN_LOG=""
INTERRUPTED=""
WORKSPACE_ROOT=""
BATCH_OUTCOME=""

log() {
  printf '[autonomous-loop] %s\n' "$*"
}

die() {
  local message="$1" code="${2:-1}"
  printf '[autonomous-loop] STOP: %s\n' "$message" >&2
  exit "$code"
}

cleanup() {
  if [[ -n "$LOCK_DIR" && -d "$LOCK_DIR" && -f "$LOCK_DIR/owner" ]]; then
    if [[ "$(<"$LOCK_DIR/owner")" == "$LOCK_TOKEN" ]]; then
      rm -f "$LOCK_DIR/owner"
      rmdir "$LOCK_DIR" 2>/dev/null || true
    fi
  fi
  [[ -z "$CHILD_DIR" ]] || rm -rf "$CHILD_DIR"
  [[ -z "$TMP_DIR" ]] || rm -rf "$TMP_DIR"
  if [[ -n "$WORKSPACE_ROOT" ]]; then
    archive_batch_worker_logs
    apply_workspace_retention
  fi
}
trap cleanup EXIT

handle_interrupt() {
  INTERRUPTED="$1"
  [[ -z "$RUN_LOG" ]] || printf '[autonomous-loop] interrupted by %s\n' "$1" >>"$RUN_LOG"
  if [[ -n "$WORKSPACE_ROOT" ]]; then
    terminate_batch_workers
  fi
  if [[ -n "$ACTIVE_PGID" ]]; then
    terminate_process_group "$ACTIVE_PGID"
    return
  fi
  case "$1" in
    HUP) exit 129 ;;
    INT) exit 130 ;;
    TERM) exit 143 ;;
  esac
}

trap 'handle_interrupt INT' INT
trap 'handle_interrupt TERM' TERM
trap 'handle_interrupt HUP' HUP

usage() {
  printf '%s\n' \
    'Usage: scripts/autonomous-loop/run.sh [--backend <claude|opencode>] [--model <model>] [--effort <level>] [--timeout <duration>] [--dry-run] [--max-iterations <n>]' \
    '                                      [--parallel <n> [--clone-depth <positive|full>] [--keep-workspaces <never|failure|always>]]' \
    '       scripts/autonomous-loop/run.sh --resume-delivery <bundle-path>' \
    '       scripts/autonomous-loop/run.sh --check-queue' \
    '' \
    'Run a finite source-checkout task loop from scripts/autonomous-loop/queue.tsv.'
}

while (($#)); do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --check-queue)
      CHECK_QUEUE=1
      shift
      ;;
    --timeout)
      (($# >= 2)) || die "--timeout requires a positive duration such as 30m or 2h" 2
      TIMEOUT="$2"
      shift 2
      ;;
    --resume-delivery)
      (($# >= 2)) || die "--resume-delivery requires a bundle path" 2
      RESUME_BUNDLE="$2"
      shift 2
      ;;
    --backend)
      (($# >= 2)) || die "--backend requires claude or opencode" 2
      case "$2" in
        claude|opencode) ;;
        *) die "invalid backend '$2': expected claude or opencode" 2 ;;
      esac
      if [[ $BACKEND_SET -eq 1 && "$BACKEND" != "$2" ]]; then
        die "conflicting --backend values: $BACKEND and $2" 2
      fi
      BACKEND="$2"
      BACKEND_SET=1
      shift 2
      ;;
    --model)
      (($# >= 2)) && [[ -n "$2" && "$2" != --* ]] || die "--model requires a model" 2
      if [[ $MODEL_SET -eq 1 && "$MODEL" != "$2" ]]; then
        die "conflicting --model values: $MODEL and $2" 2
      fi
      MODEL="$2"
      MODEL_SET=1
      shift 2
      ;;
    --effort)
      (($# >= 2)) && [[ -n "$2" && "$2" != --* ]] || die "--effort requires a level" 2
      if [[ $EFFORT_SET -eq 1 && "$EFFORT" != "$2" ]]; then
        die "conflicting --effort values: $EFFORT and $2" 2
      fi
      EFFORT="$2"
      EFFORT_SET=1
      shift 2
      ;;
    -n|--max-iterations)
      (($# >= 2)) || die "$1 requires a positive integer" 2
      MAX_ITERATIONS="$2"
      shift 2
      ;;
    --parallel)
      (($# >= 2)) && [[ "$2" =~ ^[1-9][0-9]*$ ]] || die "--parallel requires a positive integer" 2
      if [[ $PARALLEL_SET -eq 1 && "$PARALLEL" != "$2" ]]; then
        die "conflicting --parallel values: $PARALLEL and $2" 2
      fi
      PARALLEL="$2"
      PARALLEL_SET=1
      shift 2
      ;;
    --clone-depth)
      (($# >= 2)) && [[ "$2" == "full" || "$2" =~ ^[1-9][0-9]*$ ]] || \
        die "--clone-depth requires a positive integer or full" 2
      if [[ $CLONE_DEPTH_SET -eq 1 && "$CLONE_DEPTH" != "$2" ]]; then
        die "conflicting --clone-depth values: $CLONE_DEPTH and $2" 2
      fi
      CLONE_DEPTH="$2"
      CLONE_DEPTH_SET=1
      shift 2
      ;;
    --keep-workspaces)
      (($# >= 2)) || die "--keep-workspaces requires never, failure, or always" 2
      case "$2" in
        never|failure|always) ;;
        *) die "invalid --keep-workspaces value '$2': expected never, failure, or always" 2 ;;
      esac
      if [[ $KEEP_WORKSPACES_SET -eq 1 && "$KEEP_WORKSPACES" != "$2" ]]; then
        die "conflicting --keep-workspaces values: $KEEP_WORKSPACES and $2" 2
      fi
      KEEP_WORKSPACES="$2"
      KEEP_WORKSPACES_SET=1
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1" 2
      ;;
  esac
done

[[ "$MAX_ITERATIONS" =~ ^[1-9][0-9]*$ ]] || die "--max-iterations must be a positive integer" 2
[[ $CHECK_QUEUE -eq 0 || $DRY_RUN -eq 0 ]] || die "--check-queue and --dry-run are mutually exclusive" 2
# The frozen effective width composes --parallel with the remaining iteration
# budget; width 1 must stay byte-identical to the sequential invocation.
EFFECTIVE_WIDTH=$((MAX_ITERATIONS < PARALLEL ? MAX_ITERATIONS : PARALLEL))
if ((EFFECTIVE_WIDTH == 1)); then
  [[ $CLONE_DEPTH_SET -eq 0 ]] || die "--clone-depth requires an effective parallel width greater than 1" 2
  [[ $KEEP_WORKSPACES_SET -eq 0 ]] || die "--keep-workspaces requires an effective parallel width greater than 1" 2
else
  [[ $CHECK_QUEUE -eq 0 ]] || die "--check-queue and --parallel are mutually exclusive" 2
fi

parse_duration() {
  local value="$1" digits amount unit multiplier
  [[ "$value" =~ ^([1-9][0-9]*)(s|m|h)$ ]] || return 1
  digits="${BASH_REMATCH[1]}"
  ((${#digits} <= 6)) || return 1
  amount=$((10#$digits))
  unit="${BASH_REMATCH[2]}"
  case "$unit" in
    s) multiplier=1 ;;
    m) multiplier=60 ;;
    h) multiplier=3600 ;;
  esac
  TIMEOUT_SECONDS=$((amount * multiplier))
  ((TIMEOUT_SECONDS > 0 && TIMEOUT_SECONDS <= 604800))
}

parse_duration "$TIMEOUT" || die "--timeout must be a positive duration using s, m, or h" 2
if [[ -n "$RESUME_BUNDLE" && ($DRY_RUN -eq 1 || $CHECK_QUEUE -eq 1 || $BACKEND_SET -eq 1 || "$MAX_ITERATIONS" != "1" || "$TIMEOUT" != "2h" || $PARALLEL_SET -eq 1 || $CLONE_DEPTH_SET -eq 1 || $KEEP_WORKSPACES_SET -eq 1) ]]; then
  die "--resume-delivery cannot be combined with execution options" 2
fi

source "$LOOP_DIR/queue.sh"

remote_main() {
  local output
  output="$(git ls-remote --refs origin refs/heads/main 2>/dev/null)" || return 1
  [[ -n "$output" && "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" == "1" ]] || return 1
  printf '%s\n' "${output%%[[:space:]]*}"
}

assert_fresh_binary() {
  local fresh="$TMP_DIR/taskrail-fresh" cache="$TMP_DIR/gocache"
  [[ -x "$ROOT/bin/taskrail" ]] || die "working-tree binary missing: run 'task taskrail:install'" 2
  command -v mise >/dev/null 2>&1 || die "mise is required to verify the working-tree binary" 2
  mkdir -p "$cache"
  rm -f "$fresh"
  if ! CGO_ENABLED=0 GOCACHE="$cache" mise exec -- go build -trimpath -o "$fresh" ./cmd/taskrail >/dev/null; then
    die "failed to build a fresh Taskrail binary" 2
  fi
  cmp -s "$fresh" "$ROOT/bin/taskrail" || die "working-tree binary is stale: run 'task taskrail:install'" 2
}

preflight() {
  local top branch head remote current_task file status
  cd "$ROOT" || die "cannot enter repository root" 2
  top="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not a Git worktree" 2
  [[ "$top" == "$ROOT" ]] || die "script root does not equal Git worktree root" 2
  [[ "$(git rev-parse --is-bare-repository)" == "false" ]] || die "bare repositories are unsupported" 2
  branch="$(git symbolic-ref --quiet --short HEAD 2>/dev/null)" || die "detached or unborn HEAD is unsupported" 2
  [[ "$branch" == "main" ]] || die "expected branch main, found $branch" 2
  [[ -z "$(git status --porcelain=v1 --untracked-files=all --ignore-submodules=none)" ]] || die "working tree is not clean" 2
  head="$(git rev-parse HEAD)" || die "cannot resolve HEAD" 2
  remote="$(remote_main)" || die "cannot resolve exactly one origin/main" 2
  [[ "$head" == "$remote" ]] || die "local main is not exactly aligned with origin/main" 2

  current_task="$(awk '$1 == "current_task:" { value = $2; if (value == "\"\"") value = ""; print value; exit }' planning/STATE.md)"
  [[ -z "$current_task" ]] || die "STATE.md has active work: $current_task" 2
  shopt -s nullglob
  for file in planning/tasks/*.md; do
    status="$(awk '$1 == "status:" { print $2; exit }' "$file")"
    [[ "$status" != "in_progress" ]] || die "in-progress task prevents unattended selection: ${file##*/}" 2
  done
  shopt -u nullglob

  validate_queue
  assert_fresh_binary
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "taskrail validate failed" 2
}

select_task() {
  local id status
  SELECTED_ID=""
  for id in "${QUEUE_IDS[@]}"; do
    status="$(task_field "$id" status)"
    [[ "$status" == "completed" || "$status" == "cancelled" ]] && continue
    SELECTED_ID="$id"
    return 0
  done
}

validate_selected() {
  local id="$1" status dep dep_status
  status="$(task_field "$id" status)"
  [[ "$status" == "todo" ]] || die "selected task $id has status $status; operator review is required" 2
  while IFS= read -r dep; do
    [[ -n "$dep" ]] || continue
    dep_status="$(task_field "$dep" status)"
    [[ "$dep_status" == "completed" ]] || die "$id dependency $dep has status $dep_status" 2
  done < <(task_dependencies "$id")
}

render_prompt() {
  local id="$1" key tokens
  [[ -f "$PROMPT" ]] || die "prompt missing: ${PROMPT#$ROOT/}" 2
  key="$(task_key "$id")"
  tokens="$(grep -oE '\{\{[A-Z0-9_]+\}\}' "$PROMPT" | sort -u || true)"
  [[ "$tokens" == $'{{TASK_ID}}\n{{TASK_KEY}}' ]] || die "prompt contains unknown or missing template tokens" 2
  sed -e "s|{{TASK_ID}}|$id|g" -e "s|{{TASK_KEY}}|$key|g" "$PROMPT" >"$TMP_DIR/rendered-prompt.md" || \
    die "failed to render prompt" 2
  ! grep -qE '\{\{[A-Z0-9_]+\}\}' "$TMP_DIR/rendered-prompt.md" || die "rendered prompt retains a template token" 2
}

reports_for() {
  local id="$1" report
  shopt -s nullglob
  for report in "$ROOT"/planning/artifacts/verify/"$id"/*/report.json; do
    printf '%s\n' "$report"
  done
  shopt -u nullglob
}

reports_manifest() {
  local id="$1" report
  while IFS= read -r report; do
    [[ -n "$report" ]] || continue
    printf '%s\t%s\n' "$report" "$(sha256sum "$report" | awk '{print $1}')"
  done < <(reports_for "$id")
}

assert_existing_reports_unchanged() {
  local manifest="$1" report digest current
  while IFS=$'\t' read -r report digest; do
    [[ -n "$report" ]] || continue
    [[ -f "$report" ]] || return 1
    current="$(sha256sum "$report" | awk '{print $1}')"
    [[ "$current" == "$digest" ]] || return 1
  done <"$manifest"
}

new_report_path() {
  local before="$1" id="$2" report found="" count=0
  while IFS= read -r report; do
    [[ -n "$report" ]] || continue
    if ! printf '%s\n' "$before" | grep -Fxq "$report"; then
      found="$report"
      count=$((count + 1))
    fi
  done < <(reports_for "$id")
  [[ $count -eq 1 ]] || return 1
  printf '%s\n' "$found"
}

acquire_lock() {
  local common
  common="$(git rev-parse --git-common-dir)" || die "cannot resolve Git common directory" 2
  if [[ "$common" != /* ]]; then
    common="$ROOT/$common"
  fi
  LOCK_DIR="$common/taskrail-autonomous-loop.lock"
  LOCK_TOKEN="$$-$(date -u +%Y%m%dT%H%M%SZ)-$RANDOM"
  mkdir "$LOCK_DIR" 2>/dev/null || die "autonomous loop lock already exists: $LOCK_DIR" 2
  printf '%s' "$LOCK_TOKEN" >"$LOCK_DIR/owner" || die "cannot write autonomous loop lock owner" 2
}

git_control_snapshot() {
  local common git_dir root
  common="$(git rev-parse --git-common-dir)" || return 1
  git_dir="$(git rev-parse --git-dir)" || return 1
  [[ "$common" == /* ]] || common="$ROOT/$common"
  [[ "$git_dir" == /* ]] || git_dir="$ROOT/$git_dir"
  {
    git config --show-origin --list
    git ls-files --stage -v
    for root in "$common" "$git_dir"; do
      find "$root" -type f -print0
    done | sort -zu | while IFS= read -r -d '' file; do
      [[ "$file" == "$git_dir/index" ]] && continue
      printf 'file\t%s\t' "$file"
      sha256sum "$file"
    done
  } | sha256sum | awk '{print $1}'
}

build_agent_command() {
  case "$BACKEND" in
    claude)
      agent_command=(
        claude -p --permission-mode auto --add-dir "$CHILD_DIR"
        --allowedTools "Bash($CHILD_DIR/taskrail-writer *)"
      )
      [[ -z "$MODEL" ]] || agent_command+=(--model "$MODEL")
      [[ -z "$EFFORT" ]] || agent_command+=(--effort "$EFFORT")
      ;;
    opencode)
      agent_command=(opencode run --auto)
      [[ -z "$MODEL" ]] || agent_command+=(--model "$MODEL")
      [[ -z "$EFFORT" ]] || agent_command+=(--variant "$EFFORT")
      ;;
  esac
}

write_taskrail_wrapper() {
  cat >"$CHILD_DIR/taskrail-writer" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
: "${AUTONOMOUS_TASKRAIL_BINARY:?}"
repo_root="$(cd "$(dirname "$AUTONOMOUS_TASKRAIL_BINARY")/.." && pwd -P)"
MISE_TRUSTED_CONFIG_PATHS="$repo_root/mise.toml" TASKRAIL="$AUTONOMOUS_TASKRAIL_BINARY" \
  mise exec -- task taskrail:check >/dev/null
exec "$AUTONOMOUS_TASKRAIL_BINARY" "$@"
EOF
  chmod +x "$CHILD_DIR/taskrail-writer"
}

runner_log() {
  local message="$*"
  printf '[autonomous-loop] %s\n' "$message" >&2
  [[ -z "$RUN_LOG" ]] || printf '[autonomous-loop] %s\n' "$message" >>"$RUN_LOG"
}

process_group_alive() {
  kill -0 -- "-$1" 2>/dev/null
}

terminate_process_group() {
  local pgid="$1"
  process_group_alive "$pgid" || return 0
  runner_log "terminating process group $pgid"
  kill -TERM -- "-$pgid" 2>/dev/null || true
  local waited=0
  while process_group_alive "$pgid" && ((waited < KILL_GRACE_SECONDS)); do
    sleep 1
    waited=$((waited + 1))
  done
  if process_group_alive "$pgid"; then
    runner_log "forcing process group $pgid after ${KILL_GRACE_SECONDS}s"
    kill -KILL -- "-$pgid" 2>/dev/null || true
  fi
}

run_agent() {
  local prompt="$1" output_fifo="$TMP_DIR/agent-output" timeout_marker="$TMP_DIR/timed-out"
  local child_pid tee_pid child_rc tee_rc
  rm -f "$output_fifo" "$timeout_marker"
  mkfifo "$output_fifo" || return 1
  tee "$RUN_LOG" <"$output_fifo" &
  tee_pid=$!
  setsid env MISE_TRUSTED_CONFIG_PATHS="$ROOT/mise.toml" TASKRAIL="$CHILD_DIR/taskrail-writer" \
    AUTONOMOUS_TASKRAIL_BINARY="$ROOT/bin/taskrail" \
    AUTONOMOUS_TASK_ID="$SELECTED_ID" AUTONOMOUS_COMMIT_MESSAGE_FILE="$COMMIT_MESSAGE" \
    "${agent_command[@]}" <"$prompt" >"$output_fifo" 2>&1 &
  child_pid=$!
  ACTIVE_PGID="$child_pid"
  [[ -z "${WORKER_PGID_FILE:-}" ]] || printf '%s\n' "$child_pid" >"$WORKER_PGID_FILE"
  runner_log "child pid=$child_pid backend=$BACKEND timeout=$TIMEOUT"
  setsid bash -c '
    sleep "$1"
    touch "$2"
    kill -TERM -- "-$3" 2>/dev/null || true
    sleep "$4"
    kill -KILL -- "-$3" 2>/dev/null || true
  ' _ "$TIMEOUT_SECONDS" "$timeout_marker" "$child_pid" "$KILL_GRACE_SECONDS" </dev/null >/dev/null 2>&1 &
  ACTIVE_WATCHDOG=$!

  wait "$child_pid"
  child_rc=$?
  kill -TERM -- "-$ACTIVE_WATCHDOG" 2>/dev/null || true
  wait "$ACTIVE_WATCHDOG" 2>/dev/null || true
  ACTIVE_WATCHDOG=""
  if process_group_alive "$child_pid"; then
    terminate_process_group "$child_pid"
  fi
  ACTIVE_PGID=""
  wait "$tee_pid"
  tee_rc=$?
  AGENT_RC="$child_rc"
  TEE_RC="$tee_rc"
  AGENT_TIMED_OUT=0
  [[ ! -e "$timeout_marker" ]] || AGENT_TIMED_OUT=1
  [[ -z "$INTERRUPTED" ]] || runner_log "child interrupted by $INTERRUPTED"
  ((AGENT_TIMED_OUT == 0)) || runner_log "child exceeded timeout $TIMEOUT"
}

candidate_tree() {
  local index="$TMP_DIR/candidate-index-$RANDOM"
  cp "$(git rev-parse --git-path index)" "$index" || return 1
  GIT_INDEX_FILE="$index" git add -A || return 1
  GIT_INDEX_FILE="$index" git write-tree
  rm -f "$index"
}

source "$LOOP_DIR/recovery.sh"
source "$LOOP_DIR/parallel.sh"

task_manifest() {
  local path
  for path in planning/tasks/*.md; do
    printf '%s\t%s\n' "$path" "$(sha256sum "$path" | awk '{print $1}')"
  done | sort
}

assert_other_tasks_unchanged() {
  local selected="$1" manifest="$2" path digest
  while IFS=$'\t' read -r path digest; do
    [[ -n "$path" ]] || continue
    [[ "$path" == "planning/tasks/$selected.md" ]] && continue
    [[ -f "$path" && "$(sha256sum "$path" | awk '{print $1}')" == "$digest" ]] || \
      die "$selected modified or removed unrelated task $path"
  done <"$manifest"
}

validate_followup_shape() {
  local parent="$1" followup="$2" before_manifest="$3" path new_paths parent_spec before_paths
  local -a dependencies=()
  before_paths="$TMP_DIR/task-paths-before"
  cut -f1 "$before_manifest" >"$before_paths"
  assert_other_tasks_unchanged "$parent" "$before_manifest"
  new_paths="$(comm -13 "$before_paths" <(task_manifest | cut -f1))"
  if [[ -z "$followup" ]]; then
    [[ -z "$new_paths" ]] || die "$parent created an unreported follow-up task"
    return 0
  fi
  path="planning/tasks/$followup.md"
  [[ "$new_paths" == "$path" && -f "$path" ]] || die "$parent follow-up report does not name the only new task"
  [[ "$(task_field "$followup" status)" == "todo" ]] || die "$parent follow-up must have todo status"
  parent_spec="$(task_field "$parent" spec_ref)"
  [[ "$(task_field "$followup" spec_ref)" == "$parent_spec" ]] || die "$parent follow-up must inherit spec_ref"
  mapfile -t dependencies < <(task_dependencies "$followup")
  [[ ${#dependencies[@]} -eq 1 && "${dependencies[0]}" == "$parent" ]] || die "$parent follow-up must depend only on its parent"
  ! grep -qE '^loop_(policy|reason):' "$path" || die "$parent follow-up must remain implicitly held"
}

append_followup_row() {
  local parent="$1" followup="$2"
  printf '%s\t%s\t%s\n' "$followup" "hold-operator" "Verification follow-up from $parent; operator review required" >>"$QUEUE"
  validate_queue
  runner_log "queued held follow-up: $followup"
}

accept_followup() {
  local parent="$1" followup="$2" before_manifest="$3"
  validate_followup_shape "$parent" "$followup" "$before_manifest"
  [[ -z "$followup" ]] || append_followup_row "$parent" "$followup"
}

run_iteration() {
  local id="$1" before_head before_remote before_index before_reports
  local before_git_control after_git_control reports_before_manifest
  local stamp after_status verification report result report_result followup recommendation
  local commit_subject key generated_at verification_id before_manifest

  validate_selected "$id"
  if [[ -z "$CHILD_DIR" ]]; then
    CHILD_DIR="$(mktemp -d)" || die "cannot create child exchange directory" 2
  fi
  key="$(task_key "$id")"
  render_prompt "$id"
  write_taskrail_wrapper
  before_head="$(git rev-parse HEAD)"
  before_remote="$(remote_main)" || die "cannot resolve origin/main before child"
  before_index="$(git write-tree)" || die "cannot snapshot Git index"
  before_reports="$(reports_for "$id")"
  reports_before_manifest="$TMP_DIR/reports-before"
  reports_manifest "$id" >"$reports_before_manifest"
  before_git_control="$(git_control_snapshot)" || die "cannot snapshot Git control state"
  before_manifest="$TMP_DIR/tasks-before"
  task_manifest >"$before_manifest"
  COMMIT_MESSAGE="$CHILD_DIR/commit-message"
  rm -f "$COMMIT_MESSAGE"
  mkdir -p "$RUNS_DIR"
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  RUN_LOG="$RUNS_DIR/$stamp-$id-$$.log"

  build_agent_command
  command -v "${agent_command[0]}" >/dev/null 2>&1 || die "$BACKEND CLI not found"
  command -v setsid >/dev/null 2>&1 || die "setsid is required for bounded child execution" 2

  log "starting $id with $BACKEND"
  run_agent "$TMP_DIR/rendered-prompt.md" || die "$id could not launch bounded child execution"
  [[ $TEE_RC -eq 0 ]] || die "$id log streaming failed"

  [[ "$(git symbolic-ref --quiet --short HEAD 2>/dev/null)" == "main" ]] || die "$id changed the attached branch"
  [[ "$(git rev-parse HEAD)" == "$before_head" ]] || die "$id created or changed commits; the runner owns Git delivery"
  [[ "$(remote_main)" == "$before_remote" ]] || die "$id changed origin/main"
  [[ "$(git write-tree)" == "$before_index" ]] || die "$id staged changes; the runner owns staging"
  after_git_control="$(git_control_snapshot)" || die "$id left unreadable Git control state"
  [[ "$after_git_control" == "$before_git_control" ]] || die "$id changed Git control state"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all -- scripts/autonomous-loop)" ]] || \
    die "$id modified temporary loop control files"
  [[ ! -e "$COMMIT_MESSAGE" || -f "$COMMIT_MESSAGE" ]] || die "$id wrote an invalid commit-message entry"
  [[ -d "$CHILD_DIR" ]] || die "$id deleted runner child exchange directory" 2
  [[ -f "$reports_before_manifest" && -f "$before_manifest" ]] || \
    die "$id lost private runner control manifests" 2
  if [[ $AGENT_RC -ne 0 && $AGENT_TIMED_OUT -eq 0 && -z "$INTERRUPTED" ]]; then
    die "$id $BACKEND exited $AGENT_RC; inspect ${RUN_LOG#$ROOT/}"
  fi

  assert_fresh_binary
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "$id left invalid Taskrail state"
  after_status="$(task_field "$id" status)"
  verification="$(awk '$1 == "last_verification_result:" { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }' planning/STATE.md)"
  if ! report="$(new_report_path "$before_reports" "$id")"; then
    if ((AGENT_TIMED_OUT == 1)) || [[ -n "$INTERRUPTED" ]]; then
      die "$id stopped before a recoverable terminal verification; inspect ${RUN_LOG#$ROOT/}"
    fi
    die "$id did not create exactly one new verification report"
  fi
  assert_existing_reports_unchanged "$reports_before_manifest" || die "$id changed existing verification evidence"

  result=""
  if [[ "$after_status" == "completed" ]]; then
    report_result=pass
    result="completed_pass"
  elif [[ "$after_status" == "blocked" ]]; then
    report_result=fail
    result="blocked_fail"
  elif [[ "$after_status" == "in_progress" ]]; then
    report_result=fail
    result="rework_fail"
  else
    die "$id has invalid lifecycle/verification outcome: status=$after_status verification=$verification"
  fi
  check_report "$report" "$id" "$report_result" || \
    die "$id produced an invalid $report_result verification report"
  generated_at="${REPORT_FIELDS[0]:-}"
  followup="${REPORT_FIELDS[1]:-}"
  recommendation="${REPORT_FIELDS[2]:-}"
  verification_id="${REPORT_FIELDS[3]:-}"
  [[ "$verification" == "$(verification_summary "$report_result" "$id" "$generated_at" "$verification_id")" ]] || \
    die "$id state/report verification binding does not match"

  [[ -s "$COMMIT_MESSAGE" ]] || die "$id did not publish a commit message"
  "$ROOT/scripts/check-commit-msg.sh" "$COMMIT_MESSAGE" || die "$id published an invalid commit message"
  commit_subject="$(grep -vE '^[[:space:]]*#' "$COMMIT_MESSAGE" | sed '/^[[:space:]]*$/d' | head -n 1)"
  [[ "$commit_subject" =~ \($key\)$ ]] || die "$id commit subject must end with ($key)"
  git diff --cached --quiet || die "$id left staged changes"

  if ((AGENT_TIMED_OUT == 1)) || [[ -n "$INTERRUPTED" ]]; then
    [[ -z "$followup" ]] || die "$id timed out with a follow-up that requires operator inspection"
    accept_followup "$id" "" "$before_manifest"
    create_recovery_bundle "$id" "$result" "$report" "$generated_at" "$before_head" "$before_remote" "$before_index"
    die "$id reached $result but the child did not exit cleanly; resume explicitly with --resume-delivery $RECOVERY_BUNDLE"
  fi
  [[ $AGENT_RC -eq 0 ]] || die "$id $BACKEND exited $AGENT_RC; inspect ${RUN_LOG#$ROOT/}"

  accept_followup "$id" "$followup" "$before_manifest"
  [[ -z "$recommendation" ]] || runner_log "$recommendation"
  create_recovery_bundle "$id" "$result" "$report" "$generated_at" "$before_head" "$before_remote" "$before_index"
  resume_delivery "$RECOVERY_BUNDLE" normal

  if [[ "$result" == "blocked_fail" ]]; then
    die "$id was blocked and its failing verification was committed and pushed"
  elif [[ "$result" == "rework_fail" ]]; then
    die "$id remained in progress and its failing verification was committed and pushed"
  fi
}

if [[ $CHECK_QUEUE -eq 1 ]]; then
  validate_queue
  exit 0
fi
if [[ -n "$RESUME_BUNDLE" ]]; then
  TMP_DIR="$(mktemp -d)" || die "cannot create external temporary directory" 2
  cd "$ROOT" || die "cannot enter repository root" 2
  [[ "$(git rev-parse --show-toplevel 2>/dev/null)" == "$ROOT" ]] || die "not at repository root" 2
  acquire_lock
  resume_delivery "$RESUME_BUNDLE"
  exit $?
fi
if [[ $DRY_RUN -eq 1 ]]; then
  TMP_DIR="$(mktemp -d)" || die "cannot create external temporary directory" 2
else
  ensure_recovery_root
  runtime_root="$(recovery_root)"
  [[ -e "$runtime_root" ]] || mkdir "$runtime_root" || die "cannot create runtime root" 2
  [[ -d "$runtime_root" && ! -L "$runtime_root" ]] || die "runtime root is not a private directory" 2
  chmod 700 "$runtime_root" || die "cannot secure runtime root" 2
  [[ "$(stat -c '%u' "$runtime_root")" == "$(id -u)" && "$(stat -c '%a' "$runtime_root")" == "700" ]] || \
    die "runtime root permissions are unsafe" 2
  TMP_DIR="$(mktemp -d "$runtime_root/.runtime.XXXXXX")" || die "cannot create private runtime directory" 2
  chmod 700 "$TMP_DIR" || die "cannot secure private runtime directory" 2
fi
preflight
if ((EFFECTIVE_WIDTH > 1)); then
  run_parallel_batch
  exit 0
fi
select_task
if [[ -z "$SELECTED_ID" ]]; then
  log "queue exhausted"
  exit 0
fi

if [[ "${QUEUE_MODE[$SELECTED_ID]}" != "run" ]]; then
  log "held: $SELECTED_ID (${QUEUE_MODE[$SELECTED_ID]}: ${QUEUE_REASON[$SELECTED_ID]})"
  exit 20
fi
validate_selected "$SELECTED_ID"

render_prompt "$SELECTED_ID"
if [[ $DRY_RUN -eq 1 ]]; then
  log "selected: $SELECTED_ID"
  log "backend: $BACKEND"
  log "model: ${MODEL:-backend default}"
  log "effort: ${EFFORT:-backend default}"
  log "timeout: $TIMEOUT"
  log "head: $(git rev-parse HEAD)"
  log "origin/main: $(remote_main)"
  log "binary sha256: $(sha256sum "$ROOT/bin/taskrail" | awk '{print $1}')"
  log "prompt sha256: $(sha256sum "$TMP_DIR/rendered-prompt.md" | awk '{print $1}')"
  exit 0
fi

acquire_lock
preflight

iteration=0
while ((iteration < MAX_ITERATIONS)); do
  select_task
  if [[ -z "$SELECTED_ID" ]]; then
    log "queue exhausted after $iteration iteration(s)"
    exit 0
  fi
  if [[ "${QUEUE_MODE[$SELECTED_ID]}" != "run" ]]; then
    log "held: $SELECTED_ID (${QUEUE_MODE[$SELECTED_ID]}: ${QUEUE_REASON[$SELECTED_ID]})"
    exit 20
  fi
  validate_selected "$SELECTED_ID"
  iteration=$((iteration + 1))
  run_iteration "$SELECTED_ID"
  if ((iteration < MAX_ITERATIONS)); then
    preflight
  fi
done

log "iteration limit reached: $MAX_ITERATIONS"
