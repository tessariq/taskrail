#!/usr/bin/env bash
# Sourced by run.sh. Task-file parsing and queue validation for the temporary
# source-checkout loop; see AGENTS.md in this directory for the contract.

task_file() {
  printf '%s/planning/tasks/%s.md\n' "$ROOT" "$1"
}

task_key() {
  local id="$1"
  [[ "$id" =~ ^(T-[0-9]+)(-|$) ]] || die "task id has no short task key: $id" 2
  printf '%s\n' "${BASH_REMATCH[1]}"
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
declare -A QUEUE_MODE=()
declare -A QUEUE_REASON=()
declare -A QUEUE_INDEX=()

validate_queue() {
  local header line line_no=1 id mode reason extra status spec_ref dep dep_status file open_count=0
  QUEUE_IDS=()
  QUEUE_MODE=()
  QUEUE_REASON=()
  QUEUE_INDEX=()

  [[ -f "$QUEUE" ]] || die "queue missing: ${QUEUE#$ROOT/}" 2
  IFS= read -r header <"$QUEUE" || die "queue is empty" 2
  [[ "$header" == $'task_id\tmode\treason' ]] || die "invalid queue header" 2

  while IFS= read -r line || [[ -n "$line" ]]; do
    line_no=$((line_no + 1))
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" != *$'\r'* ]] || die "queue line $line_no contains CR bytes" 2
    IFS=$'\t' read -r id mode reason extra <<<"$line"
    [[ -n "$id" && -n "$mode" && -n "$reason" && -z "${extra:-}" ]] || \
      die "queue line $line_no must contain exactly three non-empty tab-separated fields" 2
    [[ -z "${QUEUE_INDEX[$id]+x}" ]] || die "duplicate task id in queue: $id" 2
    [[ -f "$(task_file "$id")" ]] || die "queue task is missing: $id" 2
    spec_ref="$(task_field "$id" spec_ref)"
    [[ "${spec_ref%%#*}" == "specs/v0.5.0.md" ]] || die "queue task is off v0.5.0: $id" 2
    case "$mode" in
      run)
        [[ "$reason" == "-" ]] || die "run row $id must use '-' reason" 2
        # A child may not edit this directory, so a task scoped to it can only
        # ever block. Catch it here instead of spending an agent attempt on it.
        # Purely mechanical: the literal path in the task file, nothing inferred.
        status="$(task_field "$id" status)"
        if [[ "$status" != "completed" && "$status" != "cancelled" ]] &&
          grep -qF 'scripts/autonomous-loop' "$(task_file "$id")"; then
          die "run row $id is scoped to scripts/autonomous-loop, which a delegated child may not edit; make it hold-operator with a reason" 2
        fi
        ;;
      hold-operator|hold-self-removal)
        [[ "$reason" != "-" ]] || die "held row $id requires a reason" 2
        ;;
      *)
        die "invalid queue mode for $id: $mode" 2
        ;;
    esac
    QUEUE_INDEX["$id"]="${#QUEUE_IDS[@]}"
    QUEUE_IDS+=("$id")
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
