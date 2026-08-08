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
TMP_DIR=""
LOCK_DIR=""
LOCK_TOKEN=""

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
  [[ -z "$TMP_DIR" ]] || rm -rf "$TMP_DIR"
}
trap cleanup EXIT

usage() {
  printf '%s\n' \
    'Usage: scripts/autonomous-loop/run.sh [--dry-run] [--max-iterations <n>]' \
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
    -n|--max-iterations)
      (($# >= 2)) || die "$1 requires a positive integer" 2
      MAX_ITERATIONS="$2"
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

task_file() {
  printf '%s/planning/tasks/%s.md\n' "$ROOT" "$1"
}

task_field() {
  local id="$1" field="$2" file
  file="$(task_file "$id")"
  [[ -f "$file" ]] || return 0
  awk -v field="$field" '$1 == field ":" { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }' "$file"
}

task_dependencies() {
  local file
  file="$(task_file "$1")"
  [[ -f "$file" ]] || return 0
  awk '
    /^dependencies:[[:space:]]*\[\][[:space:]]*$/ { exit }
    /^dependencies:[[:space:]]*$/ { in_dependencies = 1; next }
    in_dependencies && /^[[:space:]]+-[[:space:]]+/ {
      sub(/^[[:space:]]+-[[:space:]]+/, "")
      print
      next
    }
    in_dependencies { exit }
  ' "$file"
}

declare -a QUEUE_IDS=()
declare -A QUEUE_AGENT=()
declare -A QUEUE_MODE=()
declare -A QUEUE_REASON=()
declare -A QUEUE_INDEX=()

validate_queue() {
  local header line line_no=1 id agent mode reason extra status spec_ref dep dep_status file open_count=0
  QUEUE_IDS=()
  QUEUE_AGENT=()
  QUEUE_MODE=()
  QUEUE_REASON=()
  QUEUE_INDEX=()

  [[ -f "$QUEUE" ]] || die "queue missing: ${QUEUE#$ROOT/}" 2
  IFS= read -r header <"$QUEUE" || die "queue is empty" 2
  [[ "$header" == $'task_id\tagent\tmode\treason' ]] || die "invalid queue header" 2

  while IFS= read -r line || [[ -n "$line" ]]; do
    line_no=$((line_no + 1))
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" != *$'\r'* ]] || die "queue line $line_no contains CR bytes" 2
    IFS=$'\t' read -r id agent mode reason extra <<<"$line"
    [[ -n "$id" && -n "$agent" && -n "$mode" && -n "$reason" && -z "${extra:-}" ]] || \
      die "queue line $line_no must contain exactly four non-empty tab-separated fields" 2
    [[ -z "${QUEUE_INDEX[$id]+x}" ]] || die "duplicate task id in queue: $id" 2
    [[ -f "$(task_file "$id")" ]] || die "queue task is missing: $id" 2
    spec_ref="$(task_field "$id" spec_ref)"
    [[ "${spec_ref%%#*}" == "specs/v0.5.0.md" ]] || die "queue task is off v0.5.0: $id" 2
    case "$mode" in
      run)
        [[ "$agent" == "claude" || "$agent" == "opencode" ]] || die "run row $id requires claude or opencode" 2
        [[ "$reason" == "-" ]] || die "run row $id must use '-' reason" 2
        ;;
      hold-operator|hold-self-removal)
        [[ "$agent" == "none" ]] || die "held row $id must use agent none" 2
        [[ "$reason" != "-" ]] || die "held row $id requires a reason" 2
        ;;
      *)
        die "invalid queue mode for $id: $mode" 2
        ;;
    esac
    QUEUE_INDEX["$id"]="${#QUEUE_IDS[@]}"
    QUEUE_IDS+=("$id")
    QUEUE_AGENT["$id"]="$agent"
    QUEUE_MODE["$id"]="$mode"
    QUEUE_REASON["$id"]="$reason"
  done < <(sed '1d' "$QUEUE")

  ((${#QUEUE_IDS[@]} > 0)) || die "queue contains no task rows" 2

  shopt -s nullglob
  for file in "$ROOT"/planning/tasks/*.md; do
    id="$(awk '$1 == "id:" { print $2; exit }' "$file")"
    status="$(awk '$1 == "status:" { print $2; exit }' "$file")"
    spec_ref="$(awk '$1 == "spec_ref:" { print $2; exit }' "$file")"
    if [[ "${spec_ref%%#*}" == "specs/v0.5.0.md" && "$status" != "completed" && "$status" != "cancelled" ]]; then
      open_count=$((open_count + 1))
      [[ -n "${QUEUE_INDEX[$id]+x}" ]] || die "open v0.5.0 task missing from queue: $id" 2
    fi
  done
  shopt -u nullglob

  for id in "${QUEUE_IDS[@]}"; do
    while IFS= read -r dep; do
      [[ -n "$dep" ]] || continue
      dep_status="$(task_field "$dep" status)"
      [[ -n "$dep_status" ]] || die "$id has missing dependency $dep" 2
      if [[ "$dep_status" != "completed" ]]; then
        [[ -n "${QUEUE_INDEX[$dep]+x}" ]] || die "$id has unqueued incomplete dependency $dep" 2
        ((${QUEUE_INDEX[$dep]} < ${QUEUE_INDEX[$id]})) || die "$id appears before incomplete dependency $dep" 2
      fi
    done < <(task_dependencies "$id")
  done

  log "queue valid: ${#QUEUE_IDS[@]} row(s), $open_count open v0.5.0 task(s)"
}

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

  current_task="$(awk '$1 == "current_task:" { print $2; exit }' planning/STATE.md)"
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
  local id="$1" tokens
  [[ -f "$PROMPT" ]] || die "prompt missing: ${PROMPT#$ROOT/}" 2
  tokens="$(grep -oE '\{\{[A-Z0-9_]+\}\}' "$PROMPT" | sort -u || true)"
  [[ "$tokens" == "{{TASK_ID}}" ]] || die "prompt contains unknown or missing template tokens" 2
  sed "s|{{TASK_ID}}|$id|g" "$PROMPT" >"$TMP_DIR/rendered-prompt.md" || die "failed to render prompt" 2
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
    for root in "$common" "$git_dir"; do
      find "$root" -type f -print0
    done | sort -zu | while IFS= read -r -d '' file; do
      printf 'file\t%s\t' "$file"
      sha256sum "$file"
    done
  } | sha256sum | awk '{print $1}'
}

write_taskrail_wrapper() {
  cat >"$TMP_DIR/taskrail-writer" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
: "${AUTONOMOUS_TASKRAIL_BINARY:?}"
TASKRAIL="$AUTONOMOUS_TASKRAIL_BINARY" task taskrail:check >/dev/null
exec "$AUTONOMOUS_TASKRAIL_BINARY" "$@"
EOF
  chmod +x "$TMP_DIR/taskrail-writer"
}

run_iteration() {
  local id="$1" agent="${QUEUE_AGENT[$1]}" before_head before_remote before_index before_reports
  local before_git_control after_git_control reports_before_manifest
  local stamp log_file child_rc tee_rc after_status verification report report_bytes result
  local commit_message commit_subject parent count after_head after_remote generated_at

  validate_selected "$id"
  render_prompt "$id"
  write_taskrail_wrapper
  before_head="$(git rev-parse HEAD)"
  before_remote="$(remote_main)" || die "cannot resolve origin/main before child"
  before_index="$(git write-tree)" || die "cannot snapshot Git index"
  before_reports="$(reports_for "$id")"
  reports_before_manifest="$TMP_DIR/reports-before"
  reports_manifest "$id" >"$reports_before_manifest"
  before_git_control="$(git_control_snapshot)" || die "cannot snapshot Git control state"
  commit_message="$TMP_DIR/commit-message"
  rm -f "$commit_message"
  mkdir -p "$RUNS_DIR"
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  log_file="$RUNS_DIR/$stamp-$id.log"

  case "$agent" in
    claude) agent_command=(claude -p --permission-mode acceptEdits) ;;
    opencode) agent_command=(opencode run --auto) ;;
    *) die "unsupported agent for $id: $agent" ;;
  esac
  command -v "${agent_command[0]}" >/dev/null 2>&1 || die "$agent CLI not found"

  log "starting $id with $agent"
  TASKRAIL="$TMP_DIR/taskrail-writer" AUTONOMOUS_TASKRAIL_BINARY="$ROOT/bin/taskrail" AUTONOMOUS_TASK_ID="$id" \
    AUTONOMOUS_COMMIT_MESSAGE_FILE="$commit_message" \
    "${agent_command[@]}" <"$TMP_DIR/rendered-prompt.md" 2>&1 | tee "$log_file"
  pipeline_status=("${PIPESTATUS[@]}")
  child_rc="${pipeline_status[0]}"
  tee_rc="${pipeline_status[1]}"
  [[ $tee_rc -eq 0 ]] || die "$id log streaming failed"
  [[ $child_rc -eq 0 ]] || die "$id agent exited $child_rc; inspect ${log_file#$ROOT/}"

  [[ "$(git symbolic-ref --quiet --short HEAD 2>/dev/null)" == "main" ]] || die "$id changed the attached branch"
  [[ "$(git rev-parse HEAD)" == "$before_head" ]] || die "$id created or changed commits; the runner owns Git delivery"
  [[ "$(remote_main)" == "$before_remote" ]] || die "$id changed origin/main"
  [[ "$(git write-tree)" == "$before_index" ]] || die "$id staged changes; the runner owns staging"
  after_git_control="$(git_control_snapshot)" || die "$id left unreadable Git control state"
  [[ "$after_git_control" == "$before_git_control" ]] || die "$id changed Git control state"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all -- scripts/autonomous-loop)" ]] || \
    die "$id modified temporary loop control files"
  [[ ! -e "$commit_message" || -f "$commit_message" ]] || die "$id wrote an invalid commit-message entry"

  assert_fresh_binary
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "$id left invalid Taskrail state"
  after_status="$(task_field "$id" status)"
  verification="$(awk '$1 == "last_verification_result:" { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }' planning/STATE.md)"
  report="$(new_report_path "$before_reports" "$id")" || die "$id did not create exactly one new verification report"
  assert_existing_reports_unchanged "$reports_before_manifest" || die "$id changed existing verification evidence"

  result=""
  if [[ "$after_status" == "completed" ]]; then
    generated_at="$(CGO_ENABLED=0 GOCACHE="$TMP_DIR/gocache" mise exec -- go run "$LOOP_DIR/check-report.go" "$report" "$id" pass)" || \
      die "$id produced an invalid passing verification report"
    [[ "$verification" == "pass for $id at $generated_at" ]] || die "$id state/report verification binding does not match"
    result="completed_pass"
  elif [[ "$after_status" == "blocked" ]]; then
    generated_at="$(CGO_ENABLED=0 GOCACHE="$TMP_DIR/gocache" mise exec -- go run "$LOOP_DIR/check-report.go" "$report" "$id" fail)" || \
      die "$id produced an invalid failing verification report"
    [[ "$verification" == "fail for $id at $generated_at" ]] || die "$id state/report verification binding does not match"
    result="blocked_fail"
  else
    die "$id has invalid lifecycle/verification outcome: status=$after_status verification=$verification"
  fi

  [[ -s "$commit_message" ]] || die "$id did not publish a commit message"
  validate_queue
  "$ROOT/scripts/check-commit-msg.sh" "$commit_message" || die "$id published an invalid commit message"
  commit_subject="$(grep -vE '^[[:space:]]*#' "$commit_message" | sed '/^[[:space:]]*$/d' | head -n 1)"
  [[ "$commit_subject" =~ \($id\)$ ]] || die "$id commit subject must end with ($id)"
  git diff --cached --quiet || die "$id left staged changes"

  git add -A || die "$id changes could not be staged"
  git commit -F "$commit_message" || die "$id commit failed"
  after_head="$(git rev-parse HEAD)"
  parent="$(git rev-parse HEAD^)" || die "$id commit has no single parent"
  [[ "$parent" == "$before_head" ]] || die "$id commit is not a direct child of preflight HEAD"
  count="$(git rev-list --count "$before_head..$after_head")"
  [[ "$count" == "1" ]] || die "$id produced $count commits instead of one"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all --ignore-submodules=none)" ]] || die "$id commit left a dirty tree"
  git push origin HEAD:main || die "$id push to origin/main failed"
  after_remote="$(remote_main)" || die "$id cannot resolve origin/main after push"
  [[ "$after_remote" == "$after_head" ]] || die "$id push did not publish the new HEAD"
  assert_fresh_binary
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "$id post-push validation failed"

  if [[ "$result" == "blocked_fail" ]]; then
    die "$id was blocked and its failing verification was committed and pushed"
  fi
  log "completed and pushed: $id ($after_head)"
}

TMP_DIR="$(mktemp -d)" || die "cannot create external temporary directory" 2
if [[ $CHECK_QUEUE -eq 1 ]]; then
  validate_queue
  exit 0
fi
preflight
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
  log "agent: ${QUEUE_AGENT[$SELECTED_ID]}"
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
