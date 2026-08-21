#!/usr/bin/env bash
# Sourced by run.sh. Opt-in bounded parallel batch for the temporary
# source-checkout loop: one dependency-ready frontier per invocation, private
# worker clones, serial local integration, one fast-forward publication. This
# mirrors the parallel product contract in specs/v0.5.0.md without satisfying
# any of T-333, T-334, or T-335; it is removed with this directory at
# retirement.

declare -a FRONTIER=()
declare -a EXCLUDED_ROWS=()
declare -a INTEGRATED_IDS=()
declare -a UNPUBLISHED_ROWS=()
declare -A WORKSPACE_FOR=()
BATCH_BASE_HEAD=""
BATCH_STAMP=""
INTEGRATION_WS=""
GIT_USER_NAME=""
GIT_USER_EMAIL=""

terminate_batch_workers() {
  local pgid_file pgid
  [[ -n "$WORKSPACE_ROOT" && -d "$WORKSPACE_ROOT" ]] || return 0
  for pgid_file in "$WORKSPACE_ROOT"/*/pgid; do
    [[ -f "$pgid_file" ]] || continue
    IFS= read -r pgid <"$pgid_file" || continue
    [[ -n "$pgid" ]] || continue
    terminate_process_group "$pgid"
  done
}

archive_batch_worker_logs() {
  local id ws
  [[ -n "$BATCH_STAMP" && -d "$RUNS_DIR" ]] || return 0
  for id in "${FRONTIER[@]}"; do
    ws="${WORKSPACE_FOR[$id]:-}"
    [[ -n "$ws" && -f "$ws/worker.log" ]] || continue
    cp "$ws/worker.log" "$RUNS_DIR/$BATCH_STAMP-worker-$id.log" 2>/dev/null || true
  done
}

apply_workspace_retention() {
  local id ws keep_root=0
  [[ -n "$WORKSPACE_ROOT" && -d "$WORKSPACE_ROOT" ]] || return 0
  case "$KEEP_WORKSPACES" in
    never)
      rm -rf "$WORKSPACE_ROOT"
      return 0
      ;;
    always)
      log "retained workspace root: $WORKSPACE_ROOT"
      return 0
      ;;
  esac
  if [[ "$BATCH_OUTCOME" == "success" ]]; then
    rm -rf "$WORKSPACE_ROOT"
    return 0
  fi
  for id in "${FRONTIER[@]}"; do
    ws="${WORKSPACE_FOR[$id]:-}"
    [[ -n "$ws" && -d "$ws" ]] || continue
    if batch_row_integrated "$id"; then
      rm -rf "$ws"
    else
      log "retained failed workspace: $ws ($id)"
      keep_root=1
    fi
  done
  if [[ -n "$INTEGRATION_WS" && -d "$INTEGRATION_WS" ]]; then
    log "retained integration workspace: $INTEGRATION_WS"
    keep_root=1
  fi
  ((keep_root == 1)) || rm -rf "$WORKSPACE_ROOT"
}

batch_row_integrated() {
  local id="$1" integrated
  for integrated in "${INTEGRATED_IDS[@]}"; do
    [[ "$integrated" == "$id" ]] && return 0
  done
  return 1
}

# Frontier selection reads the frozen validated queue in row order. Only `run`
# rows that are todo with completed dependencies are eligible; every other open
# row records the exact reason it was held or ineligible.
select_frontier() {
  local id status mode dep dep_status reason
  FRONTIER=()
  EXCLUDED_ROWS=()
  for id in "${QUEUE_IDS[@]}"; do
    status="$(task_field "$id" status)"
    [[ "$status" == "completed" || "$status" == "cancelled" ]] && continue
    mode="${QUEUE_MODE[$id]}"
    if [[ "$mode" != "run" ]]; then
      EXCLUDED_ROWS+=("$id"$'\t'"held ($mode: ${QUEUE_REASON[$id]})")
      continue
    fi
    if [[ "$status" != "todo" ]]; then
      EXCLUDED_ROWS+=("$id"$'\t'"status $status requires operator review")
      continue
    fi
    reason=""
    while IFS= read -r dep; do
      [[ -n "$dep" ]] || continue
      dep_status="$(task_field "$dep" status)"
      if [[ "$dep_status" != "completed" ]]; then
        reason="dependency $dep is ${dep_status:-missing}"
        break
      fi
    done < <(task_dependencies "$id")
    if [[ -n "$reason" ]]; then
      EXCLUDED_ROWS+=("$id"$'\t'"$reason")
      continue
    fi
    if ((${#FRONTIER[@]} >= EFFECTIVE_WIDTH)); then
      EXCLUDED_ROWS+=("$id"$'\t'"frontier full (effective width $EFFECTIVE_WIDTH)")
      continue
    fi
    FRONTIER+=("$id")
  done
}

batch_clone_policy() {
  if [[ "$CLONE_DEPTH" == "full" ]]; then
    printf '%s\n' '--no-local --single-branch --no-tags (full history)'
  else
    printf '%s\n' "--no-local --single-branch --no-tags --depth $CLONE_DEPTH"
  fi
}

print_batch_plan() {
  local id row index=0
  log "parallel batch dry-run"
  log "effective width: $EFFECTIVE_WIDTH (requested $PARALLEL, iteration budget $MAX_ITERATIONS)"
  log "frozen base: refs/heads/main @ $(git rev-parse HEAD)"
  log "backend: $BACKEND"
  log "model: ${MODEL:-backend default}"
  log "effort: ${EFFORT:-backend default}"
  log "timeout: $TIMEOUT"
  log "workspace root policy: invocation-private external temporary directory outside the worktree, Git directory, and planning storage"
  log "clone policy: $(batch_clone_policy)"
  log "retention policy: --keep-workspaces $KEEP_WORKSPACES"
  for id in "${FRONTIER[@]}"; do
    index=$((index + 1))
    log "frontier[$index]: $id"
  done
  ((${#FRONTIER[@]} > 0)) || log "frontier is empty"
  for row in "${EXCLUDED_ROWS[@]}"; do
    log "excluded: ${row%%$'\t'*} — ${row#*$'\t'}"
  done
}

verify_clone_isolation() {
  local id="$1" clone="$2" head common linked
  head="$(git -C "$clone" rev-parse HEAD)" || die "cannot inspect clone for $id" 2
  [[ "$head" == "$BATCH_BASE_HEAD" ]] || die "clone for $id is not at the frozen base" 2
  [[ "$(git -C "$clone" symbolic-ref --quiet --short HEAD)" == "main" ]] || \
    die "clone for $id is not attached to main" 2
  common="$(git -C "$clone" rev-parse --path-format=absolute --git-common-dir)" || \
    die "cannot resolve clone Git common directory for $id" 2
  [[ "$common" == "$clone/.git" ]] || die "clone for $id does not own a private Git common directory" 2
  [[ ! -e "$clone/.git/objects/info/alternates" ]] || die "clone for $id borrows objects via alternates" 2
  linked="$(find "$clone/.git/objects" -type f -links +1 | head -n 1)"
  [[ -z "$linked" ]] || die "clone for $id is hard-linked to the source object store: $linked" 2
  if [[ "$3" != "full" ]]; then
    [[ "$(git -C "$clone" rev-parse --is-shallow-repository)" == "true" ]] || \
      die "clone for $id is unexpectedly deep" 2
  fi
}

create_batch_clone() {
  local id="$1" ws="$2" depth="$3"
  local -a depth_args=()
  if [[ "$depth" != "full" ]]; then
    depth_args=(--depth "$depth")
  fi
  mkdir -p "$ws" || die "cannot create workspace for $id" 2
  git clone --quiet --no-local --single-branch --no-tags "${depth_args[@]}" --branch main -- \
    "$ROOT" "$ws/clone" || die "cannot create private clone for $id" 2
  verify_clone_isolation "$id" "$ws/clone" "$depth"
  if [[ "$BACKEND" == "opencode" ]]; then
    # OpenCode records the attached HEAD here before running. Seed the exact
    # marker before the worker freezes Git control state so all later changes
    # remain fail-closed instead of rejecting OpenCode's deterministic marker.
    printf '%s' "$BATCH_BASE_HEAD" >"$ws/clone/.git/opencode" || \
      die "cannot seed OpenCode Git marker for $id" 2
  fi
  mkdir -p "$ws/clone/bin" || die "cannot seed clone binary directory for $id" 2
  cp "$ROOT/bin/taskrail" "$ws/clone/bin/taskrail" || die "cannot seed clone binary for $id" 2
  chmod 755 "$ws/clone/bin/taskrail"
}

# Runs in a background subshell: rebinds the runner context to the private
# clone, executes exactly one sequential-contract child, and revalidates the
# child's outcome against clone-local snapshots. Workers never select work and
# never touch the source checkout; a worker that cannot prove a clean terminal
# outcome simply leaves no result file.
worker_main() {
  local id="$1" ws="$2"
  local before_head before_remote before_index before_git_control after_git_control
  local before_reports reports_before_manifest before_manifest
  local after_status verification report result report_result
  local followup recommendation generated_at verification_id commit_subject key

  ROOT="$ws/clone"
  LOOP_DIR="$ROOT/scripts/autonomous-loop"
  QUEUE="$LOOP_DIR/queue.tsv"
  PROMPT="$LOOP_DIR/prompt.md"
  TMP_DIR="$ws/tmp"
  CHILD_DIR="$ws/child"
  RUN_LOG="$ws/worker.log"
  WORKER_PGID_FILE="$ws/pgid"
  SELECTED_ID="$id"
  LOCK_DIR=""
  WORKSPACE_ROOT=""
  cd "$ROOT" || die "worker cannot enter clone for $id" 2
  mkdir -p "$TMP_DIR" "$CHILD_DIR" || die "worker cannot create private directories for $id" 2
  : >"$RUN_LOG"

  key="$(task_key "$id")"
  validate_selected "$id"
  render_prompt "$id"
  write_taskrail_wrapper
  before_head="$(git rev-parse HEAD)"
  [[ "$before_head" == "$BATCH_BASE_HEAD" ]] || die "$id clone is not at the frozen base" 2
  before_remote="$(remote_main)" || die "$id cannot resolve source main before child"
  before_index="$(git write-tree)" || die "$id cannot snapshot Git index"
  before_reports="$(reports_for "$id")"
  reports_before_manifest="$TMP_DIR/reports-before"
  reports_manifest "$id" >"$reports_before_manifest"
  before_git_control="$(git_control_snapshot)" || die "$id cannot snapshot Git control state"
  before_manifest="$TMP_DIR/tasks-before"
  task_manifest >"$before_manifest"
  COMMIT_MESSAGE="$CHILD_DIR/commit-message"
  build_agent_command
  command -v "${agent_command[0]}" >/dev/null 2>&1 || die "$BACKEND CLI not found"

  runner_log "worker start: $id"
  run_agent "$TMP_DIR/rendered-prompt.md" || die "$id could not launch bounded child execution"
  [[ $TEE_RC -eq 0 ]] || die "$id log streaming failed"

  [[ "$(git symbolic-ref --quiet --short HEAD 2>/dev/null)" == "main" ]] || die "$id changed the attached branch"
  [[ "$(git rev-parse HEAD)" == "$before_head" ]] || die "$id created or changed commits; the runner owns Git delivery"
  [[ "$(remote_main)" == "$before_remote" ]] || die "$id changed the source main ref"
  [[ "$(git write-tree)" == "$before_index" ]] || die "$id staged changes; the runner owns staging"
  after_git_control="$(git_control_snapshot)" || die "$id left unreadable Git control state"
  [[ "$after_git_control" == "$before_git_control" ]] || die "$id changed Git control state"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all -- scripts/autonomous-loop)" ]] || \
    die "$id modified temporary loop control files"
  [[ ! -e "$COMMIT_MESSAGE" || -f "$COMMIT_MESSAGE" ]] || die "$id wrote an invalid commit-message entry"
  [[ -d "$CHILD_DIR" ]] || die "$id deleted runner child exchange directory" 2
  [[ -f "$reports_before_manifest" && -f "$before_manifest" ]] || \
    die "$id lost private runner control manifests" 2
  if ((AGENT_TIMED_OUT == 1)); then
    die "$id exceeded timeout $TIMEOUT; batch workers are never resumed"
  fi
  [[ $AGENT_RC -eq 0 ]] || die "$id $BACKEND exited $AGENT_RC; inspect $RUN_LOG"

  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "$id left invalid Taskrail state"
  after_status="$(task_field "$id" status)"
  verification="$(awk '$1 == "last_verification_result:" { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }' planning/STATE.md)"
  report="$(new_report_path "$before_reports" "$id")" || \
    die "$id did not create exactly one new verification report"
  assert_existing_reports_unchanged "$reports_before_manifest" || die "$id changed existing verification evidence"

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

  validate_followup_shape "$id" "$followup" "$before_manifest"
  [[ -z "$recommendation" ]] || runner_log "$recommendation"

  {
    printf 'result\t%s\n' "$result"
    printf 'report\t%s\n' "${report#"$ROOT"/}"
    printf 'generated_at\t%s\n' "$generated_at"
    printf 'followup\t%s\n' "$followup"
  } >"$ws/result.tmp"
  mv "$ws/result.tmp" "$ws/result" || die "$id cannot publish worker result"
  runner_log "worker done: $id ($result)"
}

worker_result_field() {
  local ws="$1" field="$2"
  awk -F '\t' -v field="$field" '$1 == field { print $2; exit }' "$ws/result"
}

worker_failure_reason() {
  local ws="$1" reason
  reason="$(grep -E '^\[autonomous-loop\] STOP: ' "$ws/worker.out" 2>/dev/null | tail -n 1)"
  reason="${reason#\[autonomous-loop\] STOP: }"
  printf '%s\n' "${reason:-worker failed without a diagnostic; inspect $ws/worker.out}"
}

batch_git_identity() {
  GIT_USER_NAME="$(git -C "$ROOT" config user.name)" || die "source checkout has no Git user.name for batch delivery" 2
  GIT_USER_EMAIL="$(git -C "$ROOT" config user.email)" || die "source checkout has no Git user.email for batch delivery" 2
}

# One bounded integration child resolves exactly one semantic replay conflict.
# It may not drop acceptance, delete a detecting test, or integrate another
# candidate; the parent revalidates all three before committing.
run_integration_child() {
  local id="$1" iclone="$2" cand_sha="$3"
  local unmerged_list="$4" path marker

  ROOT="$iclone"
  LOOP_DIR="$ROOT/scripts/autonomous-loop"
  QUEUE="$LOOP_DIR/queue.tsv"
  TMP_DIR="$INTEGRATION_WS/tmp-$id"
  CHILD_DIR="$INTEGRATION_WS/child-$id"
  RUN_LOG="$INTEGRATION_WS/integration-$id.log"
  WORKER_PGID_FILE="$INTEGRATION_WS/pgid"
  SELECTED_ID="$id"
  LOCK_DIR=""
  WORKSPACE_ROOT=""
  cd "$ROOT" || die "cannot enter integration clone" 2
  mkdir -p "$TMP_DIR" "$CHILD_DIR" || die "cannot create integration child directories" 2
  : >"$RUN_LOG"
  write_taskrail_wrapper
  COMMIT_MESSAGE="$CHILD_DIR/commit-message"
  build_agent_command
  {
    printf '%s\n' \
      "You are resolving one Git cherry-pick conflict inside a private integration clone." \
      "" \
      "Candidate task: $id (commit $cand_sha, replayed onto the current HEAD)." \
      "Conflicted paths:"
    while IFS= read -r path; do
      [[ -n "$path" ]] || continue
      printf '%s\n' "- $path"
    done <<<"$unmerged_list"
    printf '%s\n' \
      "" \
      "Resolve every conflicted file in the worktree so both the already-integrated" \
      "work and this candidate's change survive semantically. Constraints:" \
      "- Never weaken or drop acceptance criteria from planning/tasks/$id.md." \
      "- Never delete or weaken a test that detects the conflict." \
      "- Never pull in work from any other candidate or invent new scope." \
      "- Do not run any Git command that stages, commits, or changes refs; only" \
      "  edit conflicted files. The runner owns staging and delivery." \
      "Exit zero once every conflict marker is resolved."
  } >"$TMP_DIR/integration-prompt.md"
  runner_log "integration child start: $id"
  run_agent "$TMP_DIR/integration-prompt.md" || die "$id integration child could not launch"
  [[ $TEE_RC -eq 0 ]] || die "$id integration log streaming failed"
  ((AGENT_TIMED_OUT == 0)) || die "$id integration child exceeded timeout $TIMEOUT"
  [[ $AGENT_RC -eq 0 ]] || die "$id integration child exited $AGENT_RC"
  while IFS= read -r path; do
    [[ -n "$path" ]] || continue
    [[ -f "$path" ]] || die "$id integration resolution removed $path"
    if marker="$(grep -nE '^(<{7}|={7}|>{7})' "$path" | head -n 1)" && [[ -n "$marker" ]]; then
      die "$id integration resolution left a conflict marker in $path: $marker"
    fi
    git add -- "$path" || die "$id integration resolution could not be staged"
  done <<<"$unmerged_list"
  [[ -z "$(git diff --name-only --diff-filter=U)" ]] || die "$id integration resolution left unmerged paths"
  runner_log "integration child done: $id"
}

# Serial local integration: replay one accepted candidate onto the integration
# head as exactly one commit. planning/STATE.md is never merged textually — it
# is re-projected through `taskrail repair --apply`.
replay_candidate() {
  local id="$1" ws="$2" iclone="$3"
  local wclone="$ws/clone" msg="$ws/child/commit-message" cand_sha followup
  local unmerged cand_task_bytes head_before deleted

  followup="$(worker_result_field "$ws" followup)"

  # The complete replay runs in a subshell so any die() unwinds to a non-zero
  # return: the batch then reports this row as unpublished instead of aborting
  # without a terminal report.
  (
    git -C "$wclone" add -A || die "$id candidate could not be staged" 2
    git -C "$wclone" -c user.name="$GIT_USER_NAME" -c user.email="$GIT_USER_EMAIL" \
      commit --quiet -F "$msg" || die "$id candidate commit failed" 2
    cand_sha="$(git -C "$wclone" rev-parse HEAD)"
    [[ "$(git -C "$wclone" rev-parse HEAD^)" == "$BATCH_BASE_HEAD" ]] || \
      die "$id candidate is not a direct child of the frozen base" 2
    git -C "$wclone" push --quiet "$iclone" "HEAD:refs/batch/$id" || \
      die "$id candidate transfer to the integration clone failed" 2

    ROOT="$iclone"
    LOOP_DIR="$ROOT/scripts/autonomous-loop"
    QUEUE="$LOOP_DIR/queue.tsv"
    LOCK_DIR=""
    WORKSPACE_ROOT=""
    RUN_LOG="$INTEGRATION_WS/integration.log"
    cd "$ROOT" || die "cannot enter integration clone" 2
    head_before="$(git rev-parse HEAD)"
    if ! git cherry-pick --no-commit "$cand_sha" >>"$RUN_LOG" 2>&1; then
      unmerged="$(git diff --name-only --diff-filter=U)"
      [[ -n "$unmerged" ]] || die "$id cherry-pick failed without conflicts; inspect $RUN_LOG"
      if [[ "$unmerged" == "planning/STATE.md" ]]; then
        git checkout HEAD -- planning/STATE.md || die "$id could not restore planning/STATE.md"
      else
        run_integration_child "$id" "$iclone" "$cand_sha" "$unmerged"
        # The child resolves files; acceptance and detecting tests must survive.
        cand_task_bytes="$(git show "$cand_sha:planning/tasks/$id.md")" || \
          die "$id candidate task file is unreadable"
        [[ "$(cat "planning/tasks/$id.md")" == "$cand_task_bytes" ]] || \
          die "$id integration resolution changed the candidate task file"
        while IFS= read -r deleted; do
          [[ -n "$deleted" ]] || continue
          if [[ "$deleted" == *_test.go ]] && \
            git cat-file -e "$cand_sha:$deleted" 2>/dev/null; then
            die "$id integration resolution deleted detecting test $deleted"
          fi
        done < <(git diff "$head_before" --name-only --diff-filter=D)
      fi
    fi
    git checkout HEAD -- planning/STATE.md 2>/dev/null || true
    if [[ -n "$followup" ]]; then
      append_followup_row "$id" "$followup"
    fi
    TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" repair --apply >>"$RUN_LOG" 2>&1 || \
      die "$id planning/STATE.md re-projection failed"
    TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || \
      die "$id integration state is invalid"
    git add -A || die "$id integration staging failed"
    git -c user.name="$GIT_USER_NAME" -c user.email="$GIT_USER_EMAIL" \
      commit --quiet -F "$msg" || die "$id integration commit failed"
    [[ "$(git rev-parse HEAD^)" == "$head_before" ]] || die "$id integration produced more than one commit"
    [[ -z "$(git diff "HEAD^" HEAD --name-only -- scripts/autonomous-loop | grep -v '^scripts/autonomous-loop/queue.tsv$')" ]] || \
      die "$id integration commit touches loop controls beyond the queue"
  )
}

run_batch_gate() {
  local iclone="$1"
  (
    ROOT="$iclone"
    cd "$ROOT" || die "cannot enter integration clone for the gate" 2
    export MISE_TRUSTED_CONFIG_PATHS="$ROOT/mise.toml"
    local fmt
    fmt="$(mise exec -- gofmt -l . 2>&1)" || die "batch gate could not run gofmt"
    [[ -z "$fmt" ]] || die "batch gate found unformatted files: $fmt"
    mise exec -- go vet ./... >>"$INTEGRATION_WS/gate.log" 2>&1 || die "batch gate go vet failed; inspect $INTEGRATION_WS/gate.log"
    mise exec -- go test ./... >>"$INTEGRATION_WS/gate.log" 2>&1 || die "batch gate go test failed; inspect $INTEGRATION_WS/gate.log"
    TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || die "batch gate taskrail validate failed"
    TASKRAIL="$ROOT/bin/taskrail" task check:skills >>"$INTEGRATION_WS/gate.log" 2>&1 || die "batch gate skill parity failed; inspect $INTEGRATION_WS/gate.log"
    TASKRAIL="$ROOT/bin/taskrail" task check:task-bodies >>"$INTEGRATION_WS/gate.log" 2>&1 || die "batch gate task-body check failed; inspect $INTEGRATION_WS/gate.log"
  )
}

# Publication reacquires the source checkout and refuses on any drift: no
# reset, checkout overwrite, rebase, stash, or partial publication.
publish_batch() {
  local iclone="$1" head delivery_ref="refs/batch-delivery/$BATCH_STAMP"
  cd "$ROOT" || die "cannot reenter the source checkout" 2
  [[ "$(git symbolic-ref --quiet --short HEAD 2>/dev/null)" == "main" ]] || \
    die "publication refused: source checkout left branch main"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all --ignore-submodules=none)" ]] || \
    die "publication refused: source working tree is no longer clean"
  [[ "$(git rev-parse HEAD)" == "$BATCH_BASE_HEAD" ]] || \
    die "publication refused: source HEAD moved off the frozen base"
  [[ "$(git rev-parse refs/heads/main)" == "$BATCH_BASE_HEAD" ]] || \
    die "publication refused: branch tip differs from the frozen base"
  [[ "$(remote_main)" == "$BATCH_BASE_HEAD" ]] || \
    die "publication refused: origin/main moved off the frozen base"
  git -C "$iclone" push --quiet "$ROOT" "HEAD:$delivery_ref" || \
    die "publication transfer into the source checkout failed"
  if ! git merge --ff-only --quiet "$delivery_ref" 2>>"$RUN_LOG"; then
    git update-ref -d "$delivery_ref" || true
    die "publication refused: fast-forward to the integration head failed"
  fi
  git update-ref -d "$delivery_ref" || die "cannot remove the transient delivery ref"
  head="$(git rev-parse HEAD)"
  [[ -z "$(git status --porcelain=v1 --untracked-files=all --ignore-submodules=none)" ]] || \
    die "publication left a dirty source tree"
  TASKRAIL="$ROOT/bin/taskrail" "$ROOT/bin/taskrail" validate >/dev/null || \
    die "publication pre-push validation failed"
  git push origin "$head:main" || die "publication push failed"
  [[ "$(remote_main)" == "$head" ]] || die "publication push did not publish the expected commit"
  runner_log "published batch head: $head"
}

run_parallel_batch() {
  local id ws index=0 rc row iclone failed_workers=0 result reason
  local -a worker_pids=() worker_ids=()

  select_frontier
  [[ "$(git rev-parse refs/heads/main)" == "$(git rev-parse HEAD)" ]] || \
    die "branch tip differs from HEAD; refusing a parallel batch" 2

  if [[ $DRY_RUN -eq 1 ]]; then
    print_batch_plan
    return 0
  fi

  if ((${#FRONTIER[@]} == 0)); then
    if ((${#EXCLUDED_ROWS[@]} == 0)); then
      log "queue exhausted"
      return 0
    fi
    for row in "${EXCLUDED_ROWS[@]}"; do
      log "excluded: ${row%%$'\t'*} — ${row#*$'\t'}"
    done
    die "no batch-eligible run rows; operator review is required" 20
  fi

  acquire_lock
  preflight
  select_frontier
  ((${#FRONTIER[@]} > 0)) || die "frontier emptied between preflights" 2
  batch_git_identity
  command -v setsid >/dev/null 2>&1 || die "setsid is required for bounded child execution" 2
  build_agent_command
  command -v "${agent_command[0]}" >/dev/null 2>&1 || die "$BACKEND CLI not found"

  BATCH_BASE_HEAD="$(git rev-parse HEAD)"
  BATCH_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
  mkdir -p "$RUNS_DIR"
  RUN_LOG="$RUNS_DIR/$BATCH_STAMP-batch-$$.log"
  runner_log "parallel batch: width ${#FRONTIER[@]} of requested $PARALLEL at base $BATCH_BASE_HEAD"

  WORKSPACE_ROOT="$(mktemp -d)" || die "cannot create batch workspace root" 2
  WORKSPACE_ROOT="$(cd "$WORKSPACE_ROOT" && pwd -P)" || die "cannot canonicalize batch workspace root" 2
  chmod 700 "$WORKSPACE_ROOT"

  # Every clone is created and proven isolated before any worker launches, so
  # precondition failures refuse the batch without contacting a backend.
  for id in "${FRONTIER[@]}"; do
    index=$((index + 1))
    ws="$WORKSPACE_ROOT/$index-$id"
    WORKSPACE_FOR["$id"]="$ws"
    create_batch_clone "$id" "$ws" "$CLONE_DEPTH"
  done

  for id in "${FRONTIER[@]}"; do
    ws="${WORKSPACE_FOR[$id]}"
    ( worker_main "$id" "$ws" ) >"$ws/worker.out" 2>&1 &
    worker_pids+=("$!")
    worker_ids+=("$id")
    runner_log "worker launched: $id (pid ${worker_pids[-1]})"
  done

  for index in "${!worker_pids[@]}"; do
    wait "${worker_pids[$index]}"
    rc=$?
    id="${worker_ids[$index]}"
    ws="${WORKSPACE_FOR[$id]}"
    cp "$ws/worker.log" "$RUNS_DIR/$BATCH_STAMP-worker-$id.log" 2>/dev/null || true
    if ((rc != 0)) || [[ ! -f "$ws/result" ]]; then
      failed_workers=$((failed_workers + 1))
      UNPUBLISHED_ROWS+=("$id"$'\t'"$(worker_failure_reason "$ws")")
      runner_log "worker failed: $id (rc $rc); no replacement or new frontier is launched"
    fi
  done
  [[ -z "$INTERRUPTED" ]] || die "batch interrupted by $INTERRUPTED before delivery"

  local -a accepted=()
  for id in "${FRONTIER[@]}"; do
    ws="${WORKSPACE_FOR[$id]}"
    [[ -f "$ws/result" ]] || continue
    result="$(worker_result_field "$ws" result)"
    if [[ "$result" == "completed_pass" ]]; then
      accepted+=("$id")
    else
      UNPUBLISHED_ROWS+=("$id"$'\t'"terminal outcome $result requires sequential operator delivery")
    fi
  done

  if ((${#accepted[@]} == 0)); then
    print_batch_report
    die "failed batch: zero accepted candidates"
  fi

  INTEGRATION_WS="$WORKSPACE_ROOT/integration"
  create_batch_clone "integration" "$INTEGRATION_WS" full
  iclone="$INTEGRATION_WS/clone"

  for id in "${accepted[@]}"; do
    ws="${WORKSPACE_FOR[$id]}"
    if replay_candidate "$id" "$ws" "$iclone"; then
      INTEGRATED_IDS+=("$id")
      runner_log "integrated: $id"
    else
      UNPUBLISHED_ROWS+=("$id"$'\t'"integration replay failed; inspect $INTEGRATION_WS/integration.log")
      (
        cd "$iclone" || exit 0
        git cherry-pick --abort 2>/dev/null || true
        git restore --staged --worktree -- . 2>/dev/null || true
        git clean -fdq 2>/dev/null || true
      )
      [[ -z "$(git -C "$iclone" status --porcelain=v1 --untracked-files=all)" ]] || \
        die "integration clone is unrecoverable after $id; batch stops without publication"
    fi
  done

  if ((${#INTEGRATED_IDS[@]} == 0)); then
    print_batch_report
    die "failed batch: zero integrated candidates"
  fi

  run_batch_gate "$iclone" || die "batch gate failed over the final integration head"
  runner_log "local aggregate gate: pass"
  publish_batch "$iclone"

  print_batch_report
  if ((failed_workers == 0 && ${#UNPUBLISHED_ROWS[@]} == 0)); then
    BATCH_OUTCOME="success"
    return 0
  fi
  die "partial batch: ${#INTEGRATED_IDS[@]} integrated, ${#UNPUBLISHED_ROWS[@]} unpublished"
}

print_batch_report() {
  local id row
  for id in "${INTEGRATED_IDS[@]}"; do
    log "integrated: $id"
  done
  for row in "${UNPUBLISHED_ROWS[@]}"; do
    log "unpublished: ${row%%$'\t'*} — ${row#*$'\t'}"
  done
}
