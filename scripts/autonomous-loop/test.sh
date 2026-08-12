#!/usr/bin/env bash
set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNER="$SCRIPT_DIR/run.sh"
PROMPT="$SCRIPT_DIR/prompt.md"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

failures=0

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  failures=$((failures + 1))
}

assert_contains() {
  local name="$1" value="$2" expected="$3"
  [[ "$value" == *"$expected"* ]] || fail "$name: expected '$expected' in '$value'"
}

assert_not_contains() {
  local name="$1" value="$2" unexpected="$3"
  [[ "$value" != *"$unexpected"* ]] || fail "$name: did not expect '$unexpected' in '$value'"
}

extract_bundle() {
  local line
  while IFS= read -r line; do
    case "$line" in
      *"recovery bundle: "*) printf '%s\n' "${line##*recovery bundle: }"; return ;;
    esac
  done <<<"$1"
}

assert_pids_dead() {
  local name="$1" file="$2" pid
  [[ -f "$file" ]] || { fail "$name: missing pid capture"; return; }
  while IFS=' ' read -r -a pids; do
    for pid in "${pids[@]}"; do
      kill -0 "$pid" 2>/dev/null && fail "$name: process $pid survived"
    done
  done <"$file"
}

assert_review_prompt() {
  local name="$1" value
  value="$(printf '%s\n' "$2" | tr -s '[:space:]' ' ')"
  assert_contains "$name capability selection" "$value" "inspect the available installed skills and subagents"
  assert_contains "$name specialist preference" "$value" "Prefer dedicated code-simplifier and code-reviewer capabilities"
  assert_contains "$name general fallback" "$value" "otherwise use a general-purpose fresh subagent"
  assert_contains "$name explicit lenses" "$value" "with an explicit simplification or correctness lens"
  assert_contains "$name simplification delegation" "$value" "First use one fresh subagent for a behavior-preserving simplification pass"
  assert_contains "$name correctness delegation" "$value" "Then use a separate fresh subagent for correctness review"
  assert_contains "$name ordered snapshot" "$value" "freeze the resulting snapshot"
  assert_contains "$name frozen snapshot" "$value" "frozen current implementation snapshot"
  assert_contains "$name parent applies fixes" "$value" "the parent applies fixes"
  assert_contains "$name self-review rejection" "$value" "Parent-context self-review does not satisfy either pass"
  assert_contains "$name checkpoints" "$value" "Track these concise checkpoints"
  assert_contains "$name observable outcome" "$value" "independently meaningful observable outcome"
  assert_contains "$name invariants" "$value" "affected invariants"
  assert_contains "$name fix disposition" "$value" "fix-now"
  assert_contains "$name follow-up disposition" "$value" "separate-followup"
  assert_contains "$name blocked disposition" "$value" "blocked"
  assert_contains "$name rejected disposition" "$value" "rejected"
  assert_contains "$name high-medium scope" "$value" "Fix high- and medium-severity current-scope findings"
  assert_contains "$name mandatory low" "$value" "a mandatory low is current scope"
  assert_contains "$name mutation proof" "$value" "temporarily introduce the specific regression"
  assert_contains "$name named stop" "$value" "Stopping always means the blocked path"
  assert_contains "$name review ceiling" "$value" "at most five correctness reviews in total"
  assert_contains "$name repairable passes" "$value" "Findings from reviews one through four"
  assert_contains "$name terminal clean pass" "$value" "The fifth review must be clean"
  assert_contains "$name budget rework path" "$value" "use the rework path with a reason that names the unresolved review risk"
  assert_contains "$name no budget follow-up" "$value" "Budget exhaustion never turns current work into a follow-up"
  assert_contains "$name terminal outcome set" "$value" "parent accepts only completed/pass, blocked/fail, or in-progress/fail"
  assert_contains "$name blocked follow-up" "$value" "A blocked run may create one follow-up"
  assert_contains "$name held follow-up" "$value" "parent runner always queues it as held"
  assert_contains "$name parent Git ownership" "$value" "The parent runner owns Git delivery"
  assert_contains "$name no timeout retry" "$value" "Timeout never retries"
}

create_fixture() {
  local name="$1" root="$TMP_ROOT/$1" remote="$TMP_ROOT/$1.git"
  mkdir -p "$root/scripts/autonomous-loop" "$root/scripts" "$root/planning/tasks" \
    "$root/planning/artifacts/verify" "$root/specs" "$root/bin" "$root/fake-bin" "$root/captures" \
    "$TMP_ROOT/xdg-$name"
  chmod 700 "$TMP_ROOT/xdg-$name"
  cp "$RUNNER" "$root/scripts/autonomous-loop/run.sh"
  cp "$PROMPT" "$root/scripts/autonomous-loop/prompt.md"
  cp "$SCRIPT_DIR/check-report.go" "$root/scripts/autonomous-loop/check-report.go"
  cp "$SCRIPT_DIR/recovery.sh" "$root/scripts/autonomous-loop/recovery.sh"
  cp "$SCRIPT_DIR/AGENTS.md" "$root/scripts/autonomous-loop/AGENTS.md"
  cp "$SCRIPT_DIR/CLAUDE.md" "$root/scripts/autonomous-loop/CLAUDE.md"
  cp "$SCRIPT_DIR/../check-commit-msg.sh" "$root/scripts/check-commit-msg.sh"

  printf '%s\n' 'planning/artifacts/' >"$root/.gitignore"
  printf '%s\n' '# Taskrail v0.5.0' '## Test Area' >"$root/specs/v0.5.0.md"
  printf '%s\n' 'id: T-900-fixture-task' 'status: todo' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies: []' >"$root/planning/tasks/T-900-fixture-task.md"
  printf '%s\n' 'id: T-901' 'status: todo' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies:' '    - T-900-fixture-task' >"$root/planning/tasks/T-901.md"
  printf '%s\n' 'active_spec_path: specs/v0.5.0.md' 'current_task: ""' 'last_verification_result: none' >"$root/planning/STATE.md"
  printf '%s\n' $'task_id\tmode\treason' $'T-900-fixture-task\trun\t-' $'T-901\thold-operator\ttest hold' >"$root/scripts/autonomous-loop/queue.tsv"

  cat >"$root/bin/taskrail" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "validate" ]]; then
  printf '%s\n' 'state valid'
  exit 0
fi
printf 'unexpected taskrail invocation: %s\n' "$*" >&2
exit 2
EOF

  cat >"$root/fake-bin/mise" <<'EOF'
#!/usr/bin/env bash
if [[ "$*" == *" go run "* ]]; then
  while [[ "${1:-}" != "go" ]]; do shift; done
  exec "$@"
fi
out=""
while (($#)); do
  if [[ "$1" == "-o" ]]; then
    out="$2"
    break
  fi
  shift
done
[[ -n "$out" ]] || { printf '%s\n' 'missing build output' >&2; exit 2; }
cp "${AUTONOMOUS_TEST_FRESH_BINARY:-$AUTONOMOUS_TEST_BINARY}" "$out"
EOF

  cat >"$root/fake-bin/task" <<'EOF'
#!/usr/bin/env bash
if [[ "$*" == "taskrail:check" ]]; then
  [[ ! -e "$AUTONOMOUS_TEST_ROOT/captures/freshness-fail" ]]
  exit
fi
printf 'unexpected task invocation: %s\n' "$*" >&2
exit 2
EOF

  cat >"$root/fake-bin/opencode" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "${AUTONOMOUS_TEST_BACKEND:-opencode}" >"$AUTONOMOUS_TEST_ROOT/captures/backend"
printf '%s\n' "$*" >"$AUTONOMOUS_TEST_ROOT/captures/agent-args"
cat >"$AUTONOMOUS_TEST_ROOT/captures/prompt"
printf '%s\n' "$AUTONOMOUS_TASK_ID" >>"$AUTONOMOUS_TEST_ROOT/captures/agent-invocations"
if [[ "${AUTONOMOUS_TEST_AGENT_EXIT:-0}" != "0" ]]; then
  exit "$AUTONOMOUS_TEST_AGENT_EXIT"
fi
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "git-config" ]]; then
  git config user.name Mutated
fi
case "${AUTONOMOUS_TEST_OUTCOME:-completed}" in
  blocked) status=blocked; result=fail ;;
  rework) status=in_progress; result=fail ;;
  *) status=completed; result=pass ;;
esac
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "in-progress-hang" ]]; then
  printf '%s\n' 'id: T-900-fixture-task' 'status: in_progress' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies: []' >"$AUTONOMOUS_TEST_ROOT/planning/tasks/T-900-fixture-task.md"
  printf '%s\n' 'active_spec_path: specs/v0.5.0.md' 'current_task: T-900-fixture-task' 'last_verification_result: none' >"$AUTONOMOUS_TEST_ROOT/planning/STATE.md"
  sleep 300 &
  printf '%s\n' "$$ $!" >"$AUTONOMOUS_TEST_ROOT/captures/hang-pids"
  touch "$AUTONOMOUS_TEST_ROOT/captures/hang-ready"
  wait
fi
printf '%s\n' 'id: T-900-fixture-task' "status: $status" 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies: []' >"$AUTONOMOUS_TEST_ROOT/planning/tasks/T-900-fixture-task.md"
printf '%s\n' 'active_spec_path: specs/v0.5.0.md' 'current_task:' "last_verification_result: $result for T-900-fixture-task at 2026-08-08T00:00:00Z" >"$AUTONOMOUS_TEST_ROOT/planning/STATE.md"
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "index-refresh" ]]; then
  touch -d '@1' "$AUTONOMOUS_TEST_ROOT/.gitignore"
  git status --short >/dev/null
fi
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "index-flag" ]]; then
  git update-index --assume-unchanged .gitignore
fi
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "unrelated-task" || "${AUTONOMOUS_TEST_ACTION:-}" == "unrelated-terminal-hang" ]]; then
  printf '%s\n' 'id: T-901' 'status: todo' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies: []' >"$AUTONOMOUS_TEST_ROOT/planning/tasks/T-901.md"
fi
mkdir -p "$AUTONOMOUS_TEST_ROOT/planning/artifacts/verify/T-900-fixture-task/20260808T000000Z"
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "forged-report" ]]; then
  extra=',"unexpected":"pass for T-900-fixture-task"'
elif [[ "${AUTONOMOUS_TEST_ACTION:-}" == "followup" ]]; then
  extra=',"details":"follow-up recommendation: run - independently useful fixture remediation","followup_task_id":"T-902"'
elif [[ "${AUTONOMOUS_TEST_ACTION:-}" == "inline-followup" ]]; then
  extra=',"details":"Acceptance evidenced; one separate-followup finding. follow-up recommendation: hold - operator review required","followup_task_id":"T-902"'
elif [[ "${AUTONOMOUS_TEST_ACTION:-}" == "duplicate-recommendation" ]]; then
  extra=',"details":"follow-up recommendation: run - first. follow-up recommendation: hold - second","followup_task_id":"T-902"'
elif [[ "${AUTONOMOUS_TEST_ACTION:-}" == "unsupported-recommendation" ]]; then
  extra=',"details":"reviewed. follow-up recommendation: maybe - undecided mode","followup_task_id":"T-902"'
elif [[ "${AUTONOMOUS_TEST_ACTION:-}" == "empty-recommendation" ]]; then
  extra=',"details":"reviewed. follow-up recommendation: hold - ","followup_task_id":"T-902"'
else
  extra=''
fi
printf '%s\n' "{\"schema_version\":1,\"task_id\":\"T-900-fixture-task\",\"task_title\":\"Fixture\",\"result\":\"$result\",\"summary\":\"fixture\",\"generated_at\":\"2026-08-08T00:00:00Z\",\"spec_ref\":\"specs/v0.5.0.md#test-area\",\"artifacts\":[]$extra}" >"$AUTONOMOUS_TEST_ROOT/planning/artifacts/verify/T-900-fixture-task/20260808T000000Z/report.json"
case "${AUTONOMOUS_TEST_ACTION:-}" in
  followup | unreported-followup | inline-followup | duplicate-recommendation | unsupported-recommendation | empty-recommendation) create_followup=1 ;;
  *) create_followup=0 ;;
esac
if ((create_followup == 1)); then
  printf '%s\n' 'id: T-902' 'status: todo' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies:' '    - T-900-fixture-task' >"$AUTONOMOUS_TEST_ROOT/planning/tasks/T-902.md"
fi
printf '%s\n' "test: deliver $status fixture task (T-900)" >"$AUTONOMOUS_COMMIT_MESSAGE_FILE"
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "delete-exchange" ]]; then
  rm -rf "$(dirname "$AUTONOMOUS_COMMIT_MESSAGE_FILE")"
fi
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "delete-exchange-fail" ]]; then
  rm -rf "$(dirname "$AUTONOMOUS_COMMIT_MESSAGE_FILE")"
  exit 7
fi
if [[ "${AUTONOMOUS_TEST_ACTION:-}" == "final-outcome-hang" || "${AUTONOMOUS_TEST_ACTION:-}" == "unrelated-terminal-hang" ]]; then
  sleep 300 &
  printf '%s\n' "$$ $!" >"$AUTONOMOUS_TEST_ROOT/captures/hang-pids"
  touch "$AUTONOMOUS_TEST_ROOT/captures/hang-ready"
  wait
fi
EOF

  cat >"$root/fake-bin/claude" <<'EOF'
#!/usr/bin/env bash
AUTONOMOUS_TEST_BACKEND=claude exec opencode "$@"
EOF

  chmod +x "$root/bin/taskrail" "$root/fake-bin/mise" "$root/fake-bin/task" "$root/fake-bin/opencode" \
    "$root/fake-bin/claude" "$root/scripts/autonomous-loop/run.sh" "$root/scripts/check-commit-msg.sh"

  git init -q -b main "$root"
  git -C "$root" config user.name "Loop Test"
  git -C "$root" config user.email "loop-test@example.invalid"
  git -C "$root" add .
  git -C "$root" commit -q -m 'test: initialize fixture'
  git init -q --bare "$remote"
  git -C "$root" remote add origin "$remote"
  git -C "$root" push -q -u origin main
  printf '%s\n' "$root"
}

run_fixture() {
  local root="$1" fixture_path
  shift
  fixture_path="${AUTONOMOUS_TEST_PATH:-$root/fake-bin:$PATH}"
  PATH="$fixture_path" AUTONOMOUS_TEST_ROOT="$root" XDG_STATE_HOME="$TMP_ROOT/xdg-${root##*/}" \
    AUTONOMOUS_TEST_BINARY="$root/bin/taskrail" AUTONOMOUS_TEST_AGENT_EXIT="${AUTONOMOUS_TEST_AGENT_EXIT:-0}" \
    AUTONOMOUS_TEST_OUTCOME="${AUTONOMOUS_TEST_OUTCOME:-completed}" \
    AUTONOMOUS_TEST_ACTION="${AUTONOMOUS_TEST_ACTION:-}" \
    "$root/scripts/autonomous-loop/run.sh" "$@" 2>&1
}

if [[ ! -x "$RUNNER" ]]; then
  printf 'FAIL: runner is missing or not executable: %s\n' "$RUNNER" >&2
  exit 1
fi

bash -n "$RUNNER" || fail "runner syntax"
bash -n "$0" || fail "test syntax"
valid_message="$TMP_ROOT/valid-commit-message"
slugged_message="$TMP_ROOT/slugged-commit-message"
printf '%s\n' 'test: accept short task key (T-900)' >"$valid_message"
printf '%s\n' 'test: reject slugged task id (T-900-fixture-task)' >"$slugged_message"
"$SCRIPT_DIR/../check-commit-msg.sh" "$valid_message" || fail "short task key commit message was rejected"
slugged_output="$("$SCRIPT_DIR/../check-commit-msg.sh" "$slugged_message" 2>&1)"
slugged_rc=$?
[[ $slugged_rc -ne 0 ]] || fail "slugged task id commit message was accepted"
assert_contains "slugged task id guidance" "$slugged_output" "short task key"
queue_output="$("$RUNNER" --check-queue 2>&1)"
queue_rc=$?
[[ $queue_rc -eq 0 ]] || fail "repository queue validation exited $queue_rc: $queue_output"

root="$(create_fixture dry-run)"
before_head="$(git -C "$root" rev-parse HEAD)"
before_status="$(git -C "$root" status --porcelain=v1 --untracked-files=all)"
output="$(run_fixture "$root" --dry-run)"
rc=$?
[[ $rc -eq 0 ]] || fail "dry-run exited $rc: $output"
assert_contains "dry-run selection" "$output" "selected: T-900-fixture-task"
assert_contains "dry-run default backend" "$output" "backend: claude"
assert_contains "dry-run prompt digest" "$output" "prompt sha256:"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "dry-run invoked an agent"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "dry-run moved HEAD"
[[ "$(git -C "$root" status --porcelain=v1 --untracked-files=all)" == "$before_status" ]] || fail "dry-run changed the worktree"

root="$(create_fixture successful-run)"
output="$(run_fixture "$root" --max-iterations 1)"
rc=$?
[[ $rc -eq 0 ]] || fail "successful run exited $rc: $output"
assert_contains "successful task" "$output" "completed and pushed: T-900-fixture-task"
claude_prompt="$(<"$root/captures/prompt")"
assert_contains "rendered task" "$claude_prompt" "T-900-fixture-task"
assert_contains "rendered short task key" "$claude_prompt" "(T-900)"
assert_not_contains "redundant freshness command" "$claude_prompt" 'TASKRAIL="$AUTONOMOUS_TASKRAIL_BINARY" task taskrail:check'
assert_review_prompt "shared backend prompt" "$claude_prompt"
[[ "$(<"$root/captures/backend")" == "claude" ]] || fail "default backend did not invoke Claude"
claude_args="$(<"$root/captures/agent-args")"
assert_contains "default Claude arguments" "$claude_args" "-p --permission-mode auto"
assert_contains "Claude temporary directory access" "$claude_args" "--add-dir "
assert_contains "Claude wrapper permission" "$claude_args" "--allowedTools Bash("
assert_contains "Claude wrapper path" "$claude_args" "taskrail-writer *)"
[[ "$(git -C "$root" rev-list --count HEAD~1..HEAD)" == "1" ]] || fail "successful run did not create one commit"
[[ "$(git -C "$root" rev-parse HEAD)" == "$(git --git-dir="$TMP_ROOT/successful-run.git" rev-parse refs/heads/main)" ]] || fail "successful run did not push main"
dirty="$(git -C "$root" status --porcelain=v1 --untracked-files=all)"
[[ -z "$dirty" ]] || fail "successful run left a dirty tree: $dirty"

root="$(create_fixture explicit-claude)"
output="$(run_fixture "$root" --backend claude --max-iterations 1)"
rc=$?
[[ $rc -eq 0 ]] || fail "explicit Claude run exited $rc: $output"
[[ "$(<"$root/captures/backend")" == "claude" ]] || fail "explicit Claude backend invoked the wrong CLI"

root="$(create_fixture explicit-opencode)"
output="$(run_fixture "$root" --backend opencode --max-iterations 1)"
rc=$?
[[ $rc -eq 0 ]] || fail "explicit OpenCode run exited $rc: $output"
[[ "$(<"$root/captures/backend")" == "opencode" ]] || fail "explicit OpenCode backend invoked the wrong CLI"
assert_contains "explicit OpenCode arguments" "$(<"$root/captures/agent-args")" "run --auto"
opencode_prompt="$(<"$root/captures/prompt")"
[[ "$opencode_prompt" == "$claude_prompt" ]] || fail "Claude and OpenCode received different prompts"

root="$(create_fixture invalid-backend)"
output="$(run_fixture "$root" --backend unknown)"
rc=$?
[[ $rc -eq 2 ]] || fail "invalid backend expected exit 2, got $rc"
assert_contains "invalid backend" "$output" "expected claude or opencode"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "invalid backend invoked an agent"

root="$(create_fixture missing-backend)"
output="$(run_fixture "$root" --backend)"
rc=$?
[[ $rc -eq 2 ]] || fail "missing backend expected exit 2, got $rc"
assert_contains "missing backend" "$output" "--backend requires claude or opencode"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "missing backend invoked an agent"

root="$(create_fixture missing-backend-cli)"
rm "$root/fake-bin/opencode"
git -C "$root" add fake-bin/opencode
git -C "$root" commit -q -m 'test: remove OpenCode CLI'
git -C "$root" push -q
output="$(AUTONOMOUS_TEST_PATH="$root/fake-bin:/usr/bin:/bin" run_fixture "$root" --backend opencode --max-iterations 1)"
rc=$?
[[ $rc -eq 1 ]] || fail "missing backend CLI expected exit 1, got $rc"
assert_contains "missing backend CLI" "$output" "opencode CLI not found"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "missing backend CLI invoked an agent"

root="$(create_fixture invalid-timeout)"
output="$(run_fixture "$root" --timeout 0s)"
rc=$?
[[ $rc -eq 2 ]] || fail "zero timeout expected exit 2, got $rc"
assert_contains "zero timeout" "$output" "positive duration"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "invalid timeout invoked an agent"

root="$(create_fixture conflicting-backends)"
output="$(run_fixture "$root" --backend claude --backend opencode)"
rc=$?
[[ $rc -eq 2 ]] || fail "conflicting backends expected exit 2, got $rc"
assert_contains "conflicting backends" "$output" "conflicting --backend values"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "conflicting backends invoked an agent"

root="$(create_fixture duplicate-row)"
printf '%s\n' $'T-900-fixture-task\trun\t-' >>"$root/scripts/autonomous-loop/queue.tsv"
git -C "$root" add scripts/autonomous-loop/queue.tsv
git -C "$root" commit -q -m 'test: add duplicate queue row'
git -C "$root" push -q
output="$(run_fixture "$root" --dry-run)"
rc=$?
[[ $rc -eq 2 ]] || fail "duplicate queue expected exit 2, got $rc"
assert_contains "duplicate queue" "$output" "duplicate task id"

root="$(create_fixture missing-row)"
printf '%s\n' $'task_id\tmode\treason' $'T-900-fixture-task\trun\t-' >"$root/scripts/autonomous-loop/queue.tsv"
git -C "$root" add scripts/autonomous-loop/queue.tsv
git -C "$root" commit -q -m 'test: omit open queue row'
git -C "$root" push -q
output="$(run_fixture "$root" --dry-run)"
rc=$?
[[ $rc -eq 2 ]] || fail "missing queue row expected exit 2, got $rc"
assert_contains "missing queue row" "$output" "open v0.5.0 task missing from queue: T-901"

root="$(create_fixture loop-scoped-run-row)"
printf '%s\n' 'Acceptance lives in scripts/autonomous-loop/run.sh' >>"$root/planning/tasks/T-900-fixture-task.md"
git -C "$root" add planning/tasks/T-900-fixture-task.md
git -C "$root" commit -q -m 'test: scope the run row to the loop directory'
git -C "$root" push -q
output="$(run_fixture "$root" --dry-run)"
rc=$?
[[ $rc -eq 2 ]] || fail "loop-scoped run row expected exit 2, got $rc: $output"
assert_contains "loop-scoped run row" "$output" "run row T-900-fixture-task is scoped to scripts/autonomous-loop"
assert_contains "loop-scoped run row remedy" "$output" "hold-operator"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "loop-scoped run row invoked an agent"

root="$(create_fixture loop-scoped-held-row)"
printf '%s\n' 'Acceptance lives in scripts/autonomous-loop/run.sh' >>"$root/planning/tasks/T-901.md"
git -C "$root" add planning/tasks/T-901.md
git -C "$root" commit -q -m 'test: scope the held row to the loop directory'
git -C "$root" push -q
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(run_fixture "$root")"
rc=$?
[[ $rc -eq 0 ]] || fail "loop-scoped held row expected success, got $rc: $output"
[[ "$(git -C "$root" rev-parse HEAD^)" == "$before_head" ]] || fail "loop-scoped held row did not deliver T-900"

root="$(create_fixture loop-scoped-completed-row)"
printf '%s\n' 'id: T-900-fixture-task' 'status: completed' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies: []' \
  'Acceptance lived in scripts/autonomous-loop/run.sh' >"$root/planning/tasks/T-900-fixture-task.md"
printf '%s\n' $'task_id\tmode\treason' $'T-900-fixture-task\trun\t-' $'T-901\trun\t-' >"$root/scripts/autonomous-loop/queue.tsv"
git -C "$root" add planning/tasks/T-900-fixture-task.md scripts/autonomous-loop/queue.tsv
git -C "$root" commit -q -m 'test: retain a completed loop-scoped run row'
git -C "$root" push -q
output="$(run_fixture "$root" --dry-run)"
rc=$?
[[ $rc -eq 0 ]] || fail "completed loop-scoped run row expected success, got $rc: $output"
assert_contains "completed loop-scoped run row" "$output" "T-901"

root="$(create_fixture held-row)"
printf '%s\n' $'task_id\tmode\treason' $'T-900-fixture-task\thold-operator\toperator decision' $'T-901\thold-operator\ttest hold' >"$root/scripts/autonomous-loop/queue.tsv"
git -C "$root" add scripts/autonomous-loop/queue.tsv
git -C "$root" commit -q -m 'test: hold first queue row'
git -C "$root" push -q
output="$(run_fixture "$root")"
rc=$?
[[ $rc -eq 20 ]] || fail "held row expected exit 20, got $rc: $output"
assert_contains "held row" "$output" "held: T-900-fixture-task"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "held row invoked an agent"

root="$(create_fixture dirty-tree)"
printf '%s\n' dirty >"$root/untracked.txt"
output="$(run_fixture "$root" --dry-run)"
rc=$?
[[ $rc -eq 2 ]] || fail "dirty tree expected exit 2, got $rc"
assert_contains "dirty tree" "$output" "working tree is not clean"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "dirty tree invoked an agent"

root="$(create_fixture stale-binary)"
fresh_binary="$TMP_ROOT/stale-fresh-taskrail"
cp "$root/bin/taskrail" "$fresh_binary"
printf '%s\n' '# stale bytes' >>"$root/bin/taskrail"
git -C "$root" add bin/taskrail
git -C "$root" commit -q -m 'test: commit stale binary fixture'
git -C "$root" push -q
output="$(AUTONOMOUS_TEST_FRESH_BINARY="$fresh_binary" run_fixture "$root" --dry-run)"
rc=$?
[[ $rc -eq 2 ]] || fail "stale binary expected exit 2, got $rc"
assert_contains "stale binary" "$output" "working-tree binary is stale"

root="$(create_fixture blocked-run)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_OUTCOME=blocked run_fixture "$root")"
rc=$?
[[ $rc -eq 1 ]] || fail "blocked run expected exit 1, got $rc: $output"
assert_contains "blocked run" "$output" "failing verification was committed and pushed"
[[ "$(git -C "$root" rev-parse HEAD^)" == "$before_head" ]] || fail "blocked run did not create one direct-child commit"
[[ "$(git -C "$root" rev-parse HEAD)" == "$(git --git-dir="$TMP_ROOT/blocked-run.git" rev-parse refs/heads/main)" ]] || fail "blocked run did not push its evidence"
invocations_before="$(wc -l <"$root/captures/agent-invocations")"
output="$(run_fixture "$root")"
rc=$?
[[ $rc -eq 2 ]] || fail "blocked retry expected exit 2, got $rc"
assert_contains "blocked retry" "$output" "has status blocked"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "$invocations_before" ]] || fail "blocked retry invoked the agent"

root="$(create_fixture rework-run)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_OUTCOME=rework run_fixture "$root")"
rc=$?
[[ $rc -eq 1 ]] || fail "rework run expected exit 1, got $rc: $output"
assert_contains "rework run" "$output" "remained in progress and its failing verification was committed and pushed"
[[ "$(git -C "$root" rev-parse HEAD^)" == "$before_head" ]] || fail "rework run did not create one direct-child commit"
[[ "$(git -C "$root" rev-parse HEAD)" == "$(git --git-dir="$TMP_ROOT/rework-run.git" rev-parse refs/heads/main)" ]] || fail "rework run did not push its evidence"

root="$(create_fixture deleted-exchange)"
output="$(AUTONOMOUS_TEST_ACTION=delete-exchange run_fixture "$root")"
rc=$?
[[ $rc -eq 2 ]] || fail "deleted exchange expected exit 2, got $rc: $output"
assert_contains "deleted exchange diagnostic" "$output" "deleted runner child exchange directory"
assert_not_contains "deleted exchange evidence diagnosis" "$output" "changed existing verification evidence"

root="$(create_fixture deleted-exchange-fail)"
output="$(AUTONOMOUS_TEST_ACTION=delete-exchange-fail run_fixture "$root")"
rc=$?
[[ $rc -eq 2 ]] || fail "deleted exchange failure expected exit 2, got $rc: $output"
assert_contains "deleted exchange failure diagnostic" "$output" "deleted runner child exchange directory"
assert_not_contains "deleted exchange backend diagnosis" "$output" "claude exited 7"

root="$(create_fixture git-control-mutation)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=git-config run_fixture "$root")"
rc=$?
[[ $rc -eq 1 ]] || fail "Git control mutation expected exit 1, got $rc"
assert_contains "Git control mutation" "$output" "changed Git control state"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "Git control mutation created a commit"

root="$(create_fixture index-refresh)"
output="$(AUTONOMOUS_TEST_ACTION=index-refresh run_fixture "$root" --max-iterations 1)"
rc=$?
[[ $rc -eq 0 ]] || fail "index refresh exited $rc: $output"
assert_contains "index refresh" "$output" "completed and pushed: T-900-fixture-task"

root="$(create_fixture index-flag-mutation)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=index-flag run_fixture "$root")"
rc=$?
[[ $rc -eq 1 ]] || fail "index flag mutation expected exit 1, got $rc"
assert_contains "index flag mutation" "$output" "changed Git control state"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "index flag mutation created a commit"

root="$(create_fixture forged-report)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=forged-report run_fixture "$root")"
rc=$?
[[ $rc -eq 1 ]] || fail "forged report expected exit 1, got $rc"
assert_contains "forged report" "$output" "invalid pass verification report"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "forged report created a commit"

root="$(create_fixture followup-held)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=followup run_fixture "$root")"
rc=$?
[[ $rc -eq 0 ]] || fail "held follow-up expected success, got $rc: $output"
assert_contains "held follow-up queue row" "$(<"$root/scripts/autonomous-loop/queue.tsv")" $'T-902\thold-operator\tVerification follow-up from T-900-fixture-task; operator review required'
[[ "$(wc -l <"$root/captures/agent-invocations")" == "1" ]] || fail "held follow-up launched another child"
[[ "$(git -C "$root" rev-parse HEAD^)" == "$before_head" ]] || fail "held follow-up did not share the parent commit"

root="$(create_fixture inline-followup-held)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=inline-followup run_fixture "$root")"
rc=$?
[[ $rc -eq 0 ]] || fail "inline follow-up expected success, got $rc: $output"
assert_contains "inline follow-up queue row" "$(<"$root/scripts/autonomous-loop/queue.tsv")" $'T-902\thold-operator\tVerification follow-up from T-900-fixture-task; operator review required'
assert_contains "inline follow-up recommendation" "$output" "follow-up recommendation: hold - operator review required"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "1" ]] || fail "inline follow-up launched another child"
[[ "$(git -C "$root" rev-parse HEAD^)" == "$before_head" ]] || fail "inline follow-up did not share the parent commit"
[[ "$(git -C "$root" rev-parse origin/main)" == "$(git -C "$root" rev-parse HEAD)" ]] || fail "inline follow-up did not push the owning commit"

for action in duplicate-recommendation unsupported-recommendation empty-recommendation; do
  root="$(create_fixture "$action")"
  before_head="$(git -C "$root" rev-parse HEAD)"
  before_queue="$(<"$root/scripts/autonomous-loop/queue.tsv")"
  output="$(AUTONOMOUS_TEST_ACTION="$action" run_fixture "$root")"
  rc=$?
  [[ $rc -ne 0 ]] || fail "$action unexpectedly succeeded"
  assert_contains "$action" "$output" "invalid pass verification report"
  [[ "$(<"$root/scripts/autonomous-loop/queue.tsv")" == "$before_queue" ]] || fail "$action mutated the queue"
  [[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "$action created a commit"
done

root="$(create_fixture unreported-followup)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=unreported-followup run_fixture "$root")"
rc=$?
[[ $rc -ne 0 ]] || fail "unreported follow-up unexpectedly succeeded"
assert_contains "unreported follow-up" "$output" "unreported follow-up"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "unreported follow-up created a commit"

root="$(create_fixture unrelated-task-mutation)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=unrelated-task run_fixture "$root")"
rc=$?
[[ $rc -ne 0 ]] || fail "unrelated task mutation unexpectedly succeeded"
assert_contains "unrelated task mutation" "$output" "modified or removed unrelated task"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "unrelated task mutation created a commit"

root="$(create_fixture unrelated-terminal-timeout)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=unrelated-terminal-hang run_fixture "$root" --timeout 1s)"
rc=$?
[[ $rc -ne 0 ]] || fail "unrelated terminal timeout unexpectedly succeeded"
assert_contains "unrelated terminal timeout" "$output" "modified or removed unrelated task"
[[ -z "$(extract_bundle "$output")" ]] || fail "unrelated terminal timeout published a bundle"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "unrelated terminal timeout created a commit"

root="$(create_fixture in-progress-timeout)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=in-progress-hang run_fixture "$root" --timeout 1s --max-iterations 2)"
rc=$?
[[ $rc -ne 0 ]] || fail "in-progress timeout unexpectedly succeeded"
assert_contains "in-progress timeout" "$output" "exceeded timeout 1s"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "1" ]] || fail "timeout retried the child"
assert_pids_dead "in-progress timeout" "$root/captures/hang-pids"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "in-progress timeout created a commit"

root="$(create_fixture final-outcome-timeout)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=final-outcome-hang run_fixture "$root" --timeout 1s)"
rc=$?
[[ $rc -ne 0 ]] || fail "final-outcome timeout unexpectedly delivered"
bundle="$(extract_bundle "$output")"
[[ -n "$bundle" && -f "$bundle/COMPLETE" ]] || fail "final-outcome timeout did not publish a complete bundle"
assert_contains "final-outcome timeout guidance" "$output" "--resume-delivery"
assert_pids_dead "final-outcome timeout" "$root/captures/hang-pids"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "final-outcome timeout committed automatically"
output="$(run_fixture "$root" --resume-delivery "$bundle")"
rc=$?
[[ $rc -eq 0 ]] || fail "delivery resume exited $rc: $output"
assert_contains "delivery resume" "$output" "resumed and pushed: T-900-fixture-task"
[[ "$(git -C "$root" rev-parse HEAD^)" == "$before_head" ]] || fail "delivery resume did not create one direct child"
[[ "$(git -C "$root" rev-parse HEAD)" == "$(git --git-dir="$TMP_ROOT/final-outcome-timeout.git" rev-parse refs/heads/main)" ]] || fail "delivery resume did not push"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "1" ]] || fail "delivery resume invoked an agent"

root="$(create_fixture interrupted-child)"
before_head="$(git -C "$root" rev-parse HEAD)"
fixture_path="$root/fake-bin:$PATH"
PATH="$fixture_path" AUTONOMOUS_TEST_ROOT="$root" XDG_STATE_HOME="$TMP_ROOT/xdg-interrupted-child" \
  AUTONOMOUS_TEST_BINARY="$root/bin/taskrail" AUTONOMOUS_TEST_AGENT_EXIT=0 \
  AUTONOMOUS_TEST_OUTCOME=completed AUTONOMOUS_TEST_ACTION=in-progress-hang \
  "$root/scripts/autonomous-loop/run.sh" --timeout 30s >"$TMP_ROOT/interruption-output" 2>&1 &
runner_pid=$!
for _ in {1..100}; do
  [[ -e "$root/captures/hang-ready" ]] && break
  sleep 0.05
done
[[ -e "$root/captures/hang-ready" ]] || fail "interruption fixture did not become ready"
kill -TERM "$runner_pid" 2>/dev/null || true
wait "$runner_pid"
rc=$?
[[ $rc -ne 0 ]] || fail "interrupted child unexpectedly succeeded"
output="$(<"$TMP_ROOT/interruption-output")"
assert_contains "interrupted child" "$output" "interrupted by TERM"
assert_pids_dead "interrupted child" "$root/captures/hang-pids"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "1" ]] || fail "interruption retried the child"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "interruption created a commit"

root="$(create_fixture incomplete-recovery)"
output="$(AUTONOMOUS_TEST_ACTION=final-outcome-hang run_fixture "$root" --timeout 1s)"
bundle="$(extract_bundle "$output")"
rm -f "$bundle/COMPLETE"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(run_fixture "$root" --resume-delivery "$bundle")"
rc=$?
[[ $rc -eq 2 ]] || fail "incomplete recovery expected exit 2, got $rc"
assert_contains "incomplete recovery" "$output" "incomplete recovery bundle"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "incomplete recovery changed HEAD"

root="$(create_fixture tampered-recovery)"
output="$(AUTONOMOUS_TEST_ACTION=final-outcome-hang run_fixture "$root" --timeout 1s)"
bundle="$(extract_bundle "$output")"
printf '%s\n' tampered >>"$bundle/commit-message"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(run_fixture "$root" --resume-delivery "$bundle")"
rc=$?
[[ $rc -eq 2 ]] || fail "tampered recovery expected exit 2, got $rc"
assert_contains "tampered recovery" "$output" "commit message was tampered"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "tampered recovery changed HEAD"

root="$(create_fixture dirty-committed-recovery)"
output="$(AUTONOMOUS_TEST_ACTION=final-outcome-hang run_fixture "$root" --timeout 1s)"
bundle="$(extract_bundle "$output")"
before_head="$(git -C "$root" rev-parse HEAD)"
git -C "$root" add -A
git -C "$root" commit -q -F "$bundle/commit-message"
printf '%s\n' dirty >"$root/unrelated.txt"
output="$(run_fixture "$root" --resume-delivery "$bundle")"
rc=$?
[[ $rc -ne 0 ]] || fail "dirty committed recovery unexpectedly pushed"
assert_contains "dirty committed recovery" "$output" "worktree changed before push"
[[ "$(git --git-dir="$TMP_ROOT/dirty-committed-recovery.git" rev-parse refs/heads/main)" == "$before_head" ]] || fail "dirty committed recovery moved remote main"

root="$(create_fixture wrong-message-recovery)"
output="$(AUTONOMOUS_TEST_ACTION=final-outcome-hang run_fixture "$root" --timeout 1s)"
bundle="$(extract_bundle "$output")"
before_head="$(git -C "$root" rev-parse HEAD)"
git -C "$root" add -A
git -C "$root" commit -q -m 'test: wrong same-tree message (T-900)'
output="$(run_fixture "$root" --resume-delivery "$bundle")"
rc=$?
[[ $rc -ne 0 ]] || fail "wrong-message recovery unexpectedly pushed"
assert_contains "wrong-message recovery" "$output" "commit message changed"
[[ "$(git --git-dir="$TMP_ROOT/wrong-message-recovery.git" rev-parse refs/heads/main)" == "$before_head" ]] || fail "wrong-message recovery moved remote main"

root="$(create_fixture clean-committed-recovery)"
output="$(AUTONOMOUS_TEST_ACTION=final-outcome-hang run_fixture "$root" --timeout 1s)"
bundle="$(extract_bundle "$output")"
before_head="$(git -C "$root" rev-parse HEAD)"
git -C "$root" add -A
git -C "$root" commit -q -F "$bundle/commit-message"
output="$(run_fixture "$root" --resume-delivery "$bundle")"
rc=$?
[[ $rc -eq 0 ]] || fail "clean committed recovery exited $rc: $output"
[[ "$(git --git-dir="$TMP_ROOT/clean-committed-recovery.git" rev-parse refs/heads/main)" == "$(git -C "$root" rev-parse HEAD)" ]] || fail "clean committed recovery did not push expected HEAD"

root="$(create_fixture hidden-entry-recovery)"
output="$(AUTONOMOUS_TEST_ACTION=final-outcome-hang run_fixture "$root" --timeout 1s)"
bundle="$(extract_bundle "$output")"
printf '%s\n' unsafe >"$bundle/.unexpected"
chmod 600 "$bundle/.unexpected"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(run_fixture "$root" --resume-delivery "$bundle")"
rc=$?
[[ $rc -eq 2 ]] || fail "hidden-entry recovery expected exit 2, got $rc"
assert_contains "hidden-entry recovery" "$output" "unexpected entry"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "hidden-entry recovery changed HEAD"

root="$(create_fixture child-failure)"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_AGENT_EXIT=7 run_fixture "$root")"
rc=$?
[[ $rc -eq 1 ]] || fail "child failure expected exit 1, got $rc"
assert_contains "child failure backend" "$output" "claude exited 7"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "child failure created a commit"

if ((failures)); then
  printf '%d test(s) failed\n' "$failures" >&2
  exit 1
fi

printf '%s\n' 'all autonomous loop tests passed'
