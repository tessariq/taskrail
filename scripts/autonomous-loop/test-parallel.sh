#!/usr/bin/env bash
# Sourced by test.sh: fixtures for the opt-in bounded parallel batch (T-336).
# Shares the fixture builder, assertion helpers, and failure counter.


prepare_batch_fixture() {
  local root="$1"
  printf '%s\n' 'id: T-903' 'status: todo' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies: []' >"$root/planning/tasks/T-903.md"
  printf '%s\n' 'id: T-904' 'status: todo' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies:' '    - T-900-fixture-task' >"$root/planning/tasks/T-904.md"
  printf '%s\n' 'base content' >"$root/shared.txt"
  printf '%s\n' $'task_id\tmode\treason' $'T-900-fixture-task\trun\t-' $'T-903\trun\t-' \
    $'T-904\trun\t-' $'T-901\thold-operator\ttest hold' >"$root/scripts/autonomous-loop/queue.tsv"
  git -C "$root" add .
  git -C "$root" commit -q -m 'test: add batch fixture tasks'
  git -C "$root" push -q
}

root="$(create_fixture parallel-flag-invalid)"
for bad in 0 -1 abc; do
  output="$(run_fixture "$root" --parallel "$bad" 2>&1)"
  rc=$?
  [[ $rc -eq 2 ]] || fail "--parallel $bad expected exit 2, got $rc"
  assert_contains "--parallel $bad" "$output" "--parallel requires a positive integer"
done
output="$(run_fixture "$root" --parallel 2 --parallel 3)"
rc=$?
[[ $rc -eq 2 ]] || fail "conflicting --parallel expected exit 2, got $rc"
assert_contains "conflicting --parallel" "$output" "conflicting --parallel values"
output="$(run_fixture "$root" --clone-depth 1)"
rc=$?
[[ $rc -eq 2 ]] || fail "sequential --clone-depth expected exit 2, got $rc"
assert_contains "sequential --clone-depth" "$output" "--clone-depth requires an effective parallel width greater than 1"
output="$(run_fixture "$root" --keep-workspaces always)"
rc=$?
[[ $rc -eq 2 ]] || fail "sequential --keep-workspaces expected exit 2, got $rc"
assert_contains "sequential --keep-workspaces" "$output" "--keep-workspaces requires an effective parallel width greater than 1"
output="$(run_fixture "$root" --parallel 2 --max-iterations 1 --clone-depth 1)"
rc=$?
[[ $rc -eq 2 ]] || fail "width-1 --clone-depth expected exit 2, got $rc"
assert_contains "width-1 --clone-depth" "$output" "effective parallel width greater than 1"
output="$(run_fixture "$root" --parallel 2 --max-iterations 2 --clone-depth 0)"
rc=$?
[[ $rc -eq 2 ]] || fail "invalid --clone-depth expected exit 2, got $rc"
assert_contains "invalid --clone-depth" "$output" "--clone-depth requires a positive integer or full"
output="$(run_fixture "$root" --parallel 2 --max-iterations 2 --keep-workspaces sometimes)"
rc=$?
[[ $rc -eq 2 ]] || fail "invalid --keep-workspaces expected exit 2, got $rc"
assert_contains "invalid --keep-workspaces" "$output" "expected never, failure, or always"
output="$(run_fixture "$root" --parallel 2 --max-iterations 2 --check-queue)"
rc=$?
[[ $rc -eq 2 ]] || fail "--check-queue with --parallel expected exit 2, got $rc"
assert_contains "--check-queue with --parallel" "$output" "--check-queue and --parallel are mutually exclusive"
output="$(run_fixture "$root" --parallel 2 --resume-delivery /nonexistent)"
rc=$?
[[ $rc -eq 2 ]] || fail "--parallel with --resume-delivery expected exit 2, got $rc"
assert_contains "--parallel with --resume-delivery" "$output" "cannot be combined with execution options"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "parallel flag validation invoked an agent"

root="$(create_fixture parallel-width-one)"
output="$(run_fixture "$root" --parallel 1 --max-iterations 1)"
rc=$?
[[ $rc -eq 0 ]] || fail "--parallel 1 run exited $rc: $output"
assert_contains "--parallel 1 sequential delivery" "$output" "completed and pushed: T-900-fixture-task"
assert_not_contains "--parallel 1 no batch" "$output" "parallel batch"

root="$(create_fixture batch-dry-run)"
prepare_batch_fixture "$root"
printf '%s\n' 'id: T-905' 'status: todo' 'spec_ref: specs/v0.5.0.md#test-area' 'dependencies: []' >"$root/planning/tasks/T-905.md"
printf '%s\n' $'T-905\trun\t-' >>"$root/scripts/autonomous-loop/queue.tsv"
git -C "$root" add .
git -C "$root" commit -q -m 'test: add third eligible batch task'
git -C "$root" push -q
before_head="$(git -C "$root" rev-parse HEAD)"
before_status="$(git -C "$root" status --porcelain=v1 --untracked-files=all)"
output="$(run_fixture "$root" --parallel 2 --max-iterations 3 --dry-run)"
rc=$?
[[ $rc -eq 0 ]] || fail "batch dry-run exited $rc: $output"
assert_contains "batch dry-run header" "$output" "parallel batch dry-run"
assert_contains "batch dry-run width" "$output" "effective width: 2 (requested 2, iteration budget 3)"
assert_contains "batch dry-run base" "$output" "frozen base: refs/heads/main @ $before_head"
assert_contains "batch dry-run frontier 1" "$output" "frontier[1]: T-900-fixture-task"
assert_contains "batch dry-run frontier 2" "$output" "frontier[2]: T-903"
assert_contains "batch dry-run dependency reason" "$output" "excluded: T-904 — dependency T-900-fixture-task is todo"
assert_contains "batch dry-run frontier-full reason" "$output" "excluded: T-905 — frontier full (effective width 2)"
assert_contains "batch dry-run held reason" "$output" "excluded: T-901 — held (hold-operator: test hold)"
assert_contains "batch dry-run workspace policy" "$output" "workspace root policy: invocation-private external temporary directory"
assert_contains "batch dry-run clone policy" "$output" "clone policy: --no-local --single-branch --no-tags --depth 1"
assert_contains "batch dry-run retention" "$output" "retention policy: --keep-workspaces failure"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "batch dry-run invoked an agent"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "batch dry-run moved HEAD"
[[ "$(git -C "$root" status --porcelain=v1 --untracked-files=all)" == "$before_status" ]] || fail "batch dry-run changed the worktree"
assert_not_contains "batch dry-run created workspaces" "$output" "retained"

root="$(create_fixture batch-precondition)"
prepare_batch_fixture "$root"
printf '%s\n' dirty >"$root/untracked.txt"
output="$(run_fixture "$root" --parallel 2 --max-iterations 2)"
rc=$?
[[ $rc -eq 2 ]] || fail "batch dirty precondition expected exit 2, got $rc: $output"
assert_contains "batch dirty precondition" "$output" "working tree is not clean"
[[ ! -e "$root/captures/agent-invocations" ]] || fail "batch precondition refusal invoked an agent"
assert_not_contains "batch precondition workspace" "$output" "worker launched"

root="$(create_fixture batch-all-pass)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(run_fixture "$root" --parallel 2 --max-iterations 2 --keep-workspaces always)"
rc=$?
[[ $rc -eq 0 ]] || fail "batch all-pass exited $rc: $output"
assert_contains "batch all-pass integrated first" "$output" "integrated: T-900-fixture-task"
assert_contains "batch all-pass integrated second" "$output" "integrated: T-903"
assert_not_contains "batch all-pass unpublished" "$output" "unpublished:"
[[ "$(git -C "$root" rev-list --count "$before_head..HEAD")" == "2" ]] || fail "batch all-pass did not create two commits"
first_subject="$(git -C "$root" show -s --format=%s "HEAD^")"
second_subject="$(git -C "$root" show -s --format=%s HEAD)"
assert_contains "batch replay order first" "$first_subject" "(T-900)"
assert_contains "batch replay order second" "$second_subject" "(T-903)"
[[ "$(git -C "$root" rev-parse HEAD)" == "$(git --git-dir="$TMP_ROOT/batch-all-pass.git" rev-parse refs/heads/main)" ]] || fail "batch all-pass did not push main"
dirty="$(git -C "$root" status --porcelain=v1 --untracked-files=all)"
[[ -z "$dirty" ]] || fail "batch all-pass left a dirty tree: $dirty"
assert_contains "batch STATE re-projection" "$(git -C "$root" show HEAD:planning/STATE.md)" "projection: taskrail-repair"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "2" ]] || fail "batch all-pass launched an unexpected child count"
worker_cwds="$(sort -u "$root/captures/agent-cwd")"
assert_not_contains "batch workers left the source checkout" "$worker_cwds" "$root/planning"
while IFS= read -r cwd; do
  [[ "$cwd" != "$root" ]] || fail "batch worker ran inside the source checkout"
done <<<"$worker_cwds"
gate_commands="$(<"$root/captures/gate-commands")"
assert_contains "batch gate vet" "$gate_commands" "go vet"
assert_contains "batch gate test" "$gate_commands" "go test"
assert_contains "batch gate skills" "$gate_commands" "check:skills"
assert_contains "batch gate task bodies" "$gate_commands" "check:task-bodies"
wsroot="$(printf '%s\n' "$output" | sed -n 's/.*retained workspace root: //p' | head -n 1)"
[[ -n "$wsroot" && -d "$wsroot" ]] || fail "batch all-pass did not retain workspaces under --keep-workspaces always"
clone_one="$wsroot/1-T-900-fixture-task/clone"
[[ -f "$clone_one/.git/shallow" ]] || fail "batch worker clone is not shallow"
[[ ! -e "$clone_one/.git/objects/info/alternates" ]] || fail "batch worker clone borrows objects"
linked="$(find "$clone_one/.git/objects" -type f -links +1 | head -n 1)"
[[ -z "$linked" ]] || fail "batch worker clone hard-links the source object store: $linked"
[[ "$(git -C "$clone_one" rev-parse --path-format=absolute --git-common-dir)" == "$clone_one/.git" ]] || \
  fail "batch worker clone shares a Git common directory"
rm -rf "$wsroot"

root="$(create_fixture batch-mixed)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_FAIL_TASK=T-903 run_fixture "$root" --parallel 2 --max-iterations 2)"
rc=$?
[[ $rc -eq 1 ]] || fail "batch mixed expected exit 1, got $rc: $output"
assert_contains "batch mixed containment" "$output" "no replacement or new frontier is launched"
assert_contains "batch mixed integrated" "$output" "integrated: T-900-fixture-task"
assert_contains "batch mixed unpublished" "$output" "unpublished: T-903"
assert_contains "batch mixed partial" "$output" "partial batch: 1 integrated, 1 unpublished"
[[ "$(git -C "$root" rev-list --count "$before_head..HEAD")" == "1" ]] || fail "batch mixed did not deliver exactly one commit"
assert_contains "batch mixed delivered subject" "$(git -C "$root" show -s --format=%s HEAD)" "(T-900)"
[[ "$(git -C "$root" rev-parse HEAD)" == "$(git --git-dir="$TMP_ROOT/batch-mixed.git" rev-parse refs/heads/main)" ]] || fail "batch mixed did not push the integrated commit"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "2" ]] || fail "batch mixed retried or replaced a worker"
assert_contains "batch mixed retention" "$output" "retained failed workspace:"
retained_ws="$(printf '%s\n' "$output" | sed -n 's/.*retained failed workspace: \(.*\) (T-903)$/\1/p' | head -n 1)"
[[ -n "$retained_ws" && -d "$retained_ws" ]] || fail "batch mixed did not retain the failed workspace"
if git -C "$root" show HEAD | grep -Fq "$retained_ws"; then
  fail "batch mixed committed a retained workspace path"
fi
rm -rf "$(dirname "$retained_ws")"

root="$(create_fixture batch-all-fail)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
before_remote="$(git --git-dir="$TMP_ROOT/batch-all-fail.git" rev-parse refs/heads/main)"
output="$(AUTONOMOUS_TEST_AGENT_EXIT=7 run_fixture "$root" --parallel 2 --max-iterations 2 --keep-workspaces never)"
rc=$?
[[ $rc -eq 1 ]] || fail "batch all-fail expected exit 1, got $rc: $output"
assert_contains "batch all-fail" "$output" "failed batch: zero accepted candidates"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "batch all-fail created a commit"
[[ "$(git --git-dir="$TMP_ROOT/batch-all-fail.git" rev-parse refs/heads/main)" == "$before_remote" ]] || fail "batch all-fail pushed"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "2" ]] || fail "batch all-fail retried a worker"
assert_not_contains "batch all-fail retention" "$output" "retained"

root="$(create_fixture batch-timeout)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_HANG_TASK=T-903 run_fixture "$root" --parallel 2 --max-iterations 2 --timeout 1s --keep-workspaces never)"
rc=$?
[[ $rc -eq 1 ]] || fail "batch timeout expected exit 1, got $rc: $output"
assert_contains "batch timeout integrated" "$output" "integrated: T-900-fixture-task"
assert_contains "batch timeout unpublished" "$output" "unpublished: T-903"
assert_contains "batch timeout reason" "$output" "exceeded timeout 1s"
assert_pids_dead "batch timeout" "$root/captures/hang-pids"
[[ "$(git -C "$root" rev-list --count "$before_head..HEAD")" == "1" ]] || fail "batch timeout did not deliver the surviving candidate"
[[ "$(wc -l <"$root/captures/agent-invocations")" == "2" ]] || fail "batch timeout retried a worker"

root="$(create_fixture batch-conflict)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=conflict run_fixture "$root" --parallel 2 --max-iterations 2 --keep-workspaces never)"
rc=$?
[[ $rc -eq 0 ]] || fail "batch conflict exited $rc: $output"
assert_contains "batch conflict integrated first" "$output" "integrated: T-900-fixture-task"
assert_contains "batch conflict integrated second" "$output" "integrated: T-903"
[[ "$(wc -l <"$root/captures/integration-invocations")" == "1" ]] || fail "batch conflict launched more than one integration child"
[[ "$(git -C "$root" rev-list --count "$before_head..HEAD")" == "2" ]] || fail "batch conflict did not deliver both candidates"
assert_contains "batch conflict resolution" "$(git -C "$root" show HEAD:shared.txt)" "merged shared.txt"

root="$(create_fixture batch-conflict-unresolved)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_ACTION=conflict-unresolved run_fixture "$root" --parallel 2 --max-iterations 2)"
rc=$?
[[ $rc -eq 1 ]] || fail "batch unresolved conflict expected exit 1, got $rc: $output"
assert_contains "batch unresolved conflict integrated" "$output" "integrated: T-900-fixture-task"
assert_contains "batch unresolved conflict unpublished" "$output" "unpublished: T-903"
[[ "$(wc -l <"$root/captures/integration-invocations")" == "1" ]] || fail "batch unresolved conflict launched extra integration children"
[[ "$(git -C "$root" rev-list --count "$before_head..HEAD")" == "1" ]] || fail "batch unresolved conflict did not deliver the clean candidate"

root="$(create_fixture batch-source-drift)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
before_remote="$(git --git-dir="$TMP_ROOT/batch-source-drift.git" rev-parse refs/heads/main)"
output="$(AUTONOMOUS_TEST_ACTION=source-drift run_fixture "$root" --parallel 2 --max-iterations 2)"
rc=$?
[[ $rc -ne 0 ]] || fail "batch source drift unexpectedly published"
assert_contains "batch source drift refusal" "$output" "publication refused: source working tree is no longer clean"
[[ "$(git -C "$root" rev-parse HEAD)" == "$before_head" ]] || fail "batch source drift moved HEAD"
[[ "$(git --git-dir="$TMP_ROOT/batch-source-drift.git" rev-parse refs/heads/main)" == "$before_remote" ]] || fail "batch source drift pushed"

root="$(create_fixture batch-followup)"
prepare_batch_fixture "$root"
before_head="$(git -C "$root" rev-parse HEAD)"
output="$(AUTONOMOUS_TEST_FOLLOWUP_TASK=T-900-fixture-task run_fixture "$root" --parallel 2 --max-iterations 2 --keep-workspaces never)"
rc=$?
[[ $rc -eq 0 ]] || fail "batch follow-up exited $rc: $output"
assert_contains "batch follow-up queue row" "$(git -C "$root" show HEAD:scripts/autonomous-loop/queue.tsv)" \
  $'T-902\thold-operator\tVerification follow-up from T-900-fixture-task; operator review required'
followup_commit_files="$(git -C "$root" show --name-only --format= "HEAD^")"
assert_contains "batch follow-up owning commit queue" "$followup_commit_files" "scripts/autonomous-loop/queue.tsv"
assert_contains "batch follow-up owning commit task" "$followup_commit_files" "planning/tasks/T-902.md"
second_commit_files="$(git -C "$root" show --name-only --format= HEAD)"
assert_not_contains "batch follow-up leaked into second commit" "$second_commit_files" "planning/tasks/T-902.md"
[[ "$(git -C "$root" rev-list --count "$before_head..HEAD")" == "2" ]] || fail "batch follow-up did not deliver both candidates"
