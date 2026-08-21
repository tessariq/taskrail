#!/usr/bin/env bash
# Sourced by test.sh: fixtures for the temporary parent-agent operator bridge.

OPERATOR="$SCRIPT_DIR/operator.sh"

[[ -x "$OPERATOR" ]] || fail "operator bridge is missing or not executable: $OPERATOR"
if [[ -f "$OPERATOR" ]]; then
  bash -n "$OPERATOR" || fail "operator bridge syntax"
  operator_bytes="$(<"$OPERATOR")"
  assert_not_contains "operator queue mutation" "$operator_bytes" 'queue.tsv"'
  assert_not_contains "operator cherry-pick" "$operator_bytes" "git cherry-pick"
  assert_not_contains "operator commit" "$operator_bytes" "git commit"
  assert_not_contains "operator repair" "$operator_bytes" "taskrail repair"
  assert_not_contains "operator force push" "$operator_bytes" "--force"
  assert_not_contains "operator retry" "$operator_bytes" "retry_worker"
fi

operator_fixture="$TMP_ROOT/operator"
operator_remote="$TMP_ROOT/operator.git"
mkdir -p "$operator_fixture/scripts/autonomous-loop" "$operator_fixture/fake-bin" \
  "$operator_fixture/planning/artifacts/runs" "$operator_fixture/planning/tasks" "$operator_fixture/bin"
cp "$OPERATOR" "$operator_fixture/scripts/autonomous-loop/operator.sh" 2>/dev/null || true
cp "$SCRIPT_DIR/check-report.go" "$operator_fixture/scripts/autonomous-loop/check-report.go"

cat >"$operator_fixture/scripts/autonomous-loop/run.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$AUTONOMOUS_OPERATOR_CAPTURE/runner-argv"
if [[ " $* " == *" --resume-delivery "* ]]; then
  printf '%s\n' resumed >>"$AUTONOMOUS_OPERATOR_CAPTURE/resume-invocations"
  bundle="$2"
  git add README.md planning/STATE.md
  git commit -q -F "$bundle/commit-message"
  git push -q origin main
  printf '%s\n' "$(git rev-parse HEAD)" >"$bundle/DELIVERED"
  chmod 600 "$bundle/DELIVERED"
  printf '[autonomous-loop] resumed and pushed: T-900 (%s)\n' "$(git rev-parse HEAD)"
  exit 0
fi
if [[ " $* " == *" --dry-run "* ]]; then
  printf '%s\n' \
    '[autonomous-loop] parallel batch dry-run' \
    '[autonomous-loop] effective width: 2 (requested 2, iteration budget 2)' \
    '[autonomous-loop] frozen base: refs/heads/main @ fixture-head' \
    '[autonomous-loop] frontier[1]: T-900' \
    '[autonomous-loop] frontier[2]: T-901' \
    '[autonomous-loop] excluded: T-902 - held (hold-operator: fixture)' \
    '[autonomous-loop] clone policy: --no-local --single-branch --no-tags --depth 1' \
    '[autonomous-loop] retention policy: --keep-workspaces failure'
  if [[ -n "${AUTONOMOUS_OPERATOR_DRIFT_DRY:-}" ]]; then
    count_file="$AUTONOMOUS_OPERATOR_CAPTURE/dry-count"
    count=0
    [[ ! -f "$count_file" ]] || count="$(<"$count_file")"
    count=$((count + 1))
    printf '%s\n' "$count" >"$count_file"
    ((count == 1)) || printf '%s\n' '[autonomous-loop] frozen base changed'
  fi
  exit 0
fi
if [[ -n "${AUTONOMOUS_OPERATOR_BUNDLE:-}" ]]; then
  printf '[autonomous-loop] recovery bundle: %s\n' "$AUTONOMOUS_OPERATOR_BUNDLE"
  exit 1
fi
if [[ -n "${AUTONOMOUS_OPERATOR_QUOTA_BATCH:-}" ]]; then
  printf '%s\n' \
    '[autonomous-loop] worker launched: T-900 (pid 100)' \
    '[autonomous-loop] worker launched: T-901 (pid 101)' \
    'provider stderr: quota exhausted; reset tomorrow morning' \
    '[autonomous-loop] worker failed: T-900 (rc 7); no replacement or new frontier is launched' \
    '[autonomous-loop] worker done: T-901 (completed_pass)' \
    '[autonomous-loop] integrated: T-901' \
    '[autonomous-loop] unpublished: T-900 - provider exited 7' \
    '[autonomous-loop] local aggregate gate: pass'
  exit 1
fi
if [[ -n "${AUTONOMOUS_OPERATOR_QUOTA_SEQUENTIAL:-}" ]]; then
  printf '%s\n' \
    '[autonomous-loop] starting T-900 with claude' \
    'provider stderr: quota exhausted; reset tomorrow morning'
  exit 7
fi
head="$(git rev-parse HEAD)"
printf '%s\n' \
  '[autonomous-loop] worker launched: T-900 (pid 100)' \
  '[autonomous-loop] worker launched: T-901 (pid 101)' \
  '[autonomous-loop] worker done: T-901 (completed_pass)' \
  '[autonomous-loop] worker done: T-900 (completed_pass)' \
  '[autonomous-loop] integrated: T-900' \
  '[autonomous-loop] integrated: T-901' \
  '[autonomous-loop] local aggregate gate: pass' \
  "[autonomous-loop] published batch head: $head"
EOF

cat >"$operator_fixture/fake-bin/task" <<'EOF'
#!/usr/bin/env bash
[[ "$*" == "taskrail:check" ]] || exit 2
printf '%s\n' checked >>"$AUTONOMOUS_OPERATOR_CAPTURE/freshness"
EOF

cat >"$operator_fixture/fake-bin/taskrail" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

cat >"$operator_fixture/fake-bin/claude" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

cat >"$operator_fixture/fake-bin/gh" <<'EOF'
#!/usr/bin/env bash
head="$(git rev-parse HEAD)"
printf 'CI\t%s\tcompleted\tsuccess\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\nUnrelated\t%s\tcompleted\tsuccess\t4\n' \
  "$head" "$head" "$head" 0000000000000000000000000000000000000000
EOF

chmod +x "$operator_fixture/scripts/autonomous-loop/run.sh" "$operator_fixture/scripts/autonomous-loop/operator.sh" \
  "$operator_fixture/fake-bin/task" "$operator_fixture/fake-bin/taskrail" \
  "$operator_fixture/fake-bin/claude" "$operator_fixture/fake-bin/gh"
printf '%s\n' fixture >"$operator_fixture/README.md"
printf '%s\n' 'planning/artifacts/' >"$operator_fixture/.gitignore"
printf '%s\n' 'id: T-900' 'status: completed' >"$operator_fixture/planning/tasks/T-900.md"
printf '%s\n' 'last_verification_result: none' >"$operator_fixture/planning/STATE.md"
printf '%s\n' $'task_id\tmode\treason' $'T-900\thold-operator\tfixture' >"$operator_fixture/scripts/autonomous-loop/queue.tsv"
cat >"$operator_fixture/scripts/check-commit-msg.sh" <<'EOF'
#!/usr/bin/env bash
[[ -s "$1" ]]
EOF
chmod +x "$operator_fixture/scripts/check-commit-msg.sh"
git init -q -b main "$operator_fixture"
git -C "$operator_fixture" config user.name "Operator Test"
git -C "$operator_fixture" config user.email "operator@example.invalid"
git -C "$operator_fixture" add .
git -C "$operator_fixture" commit -q -m 'test: initialize operator fixture'
git init -q --bare "$operator_remote"
git -C "$operator_fixture" remote add origin "$operator_remote"
git -C "$operator_fixture" push -q -u origin main

operator_input="$(printf '\n\n\n2\n2\n\n\n\nCI,Planning,CodeQL\n2\n0\nRUN\nno\n')"
operator_output="$(printf '%s\n' "$operator_input" | \
  PATH="$operator_fixture/fake-bin:$PATH" \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" \
  AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  "$operator_fixture/scripts/autonomous-loop/operator.sh" 2>&1)"
operator_rc=$?
[[ $operator_rc -eq 0 ]] || fail "operator happy path exited $operator_rc: $operator_output"
assert_contains "operator exact dry-run" "$operator_output" "Exact dry-run command:"
assert_contains "operator frontier explanation" "$operator_output" "frontier[1]: T-900"
assert_contains "operator authority boundary" "$operator_output" "The runner alone owns worker lifecycle, integration, Git, planning, and delivery mutations."
assert_contains "operator ranked worker" "$operator_output" "worker[1]: T-900 - completed_pass"
assert_contains "operator ranked worker two" "$operator_output" "worker[2]: T-901 - completed_pass"
assert_contains "operator integration" "$operator_output" "integration: T-900 - integrated"
assert_contains "operator local gate" "$operator_output" "local-gate: pass"
assert_contains "operator delivery" "$operator_output" "delivery: pass"
assert_contains "operator CI" "$operator_output" "remote-CI: pass"
[[ "$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")" == "3" ]] || \
  fail "operator did not recheck one dry-run and invoke exactly one live runner"
dry_argv="$(sed -n '1p' "$operator_fixture/planning/artifacts/runs/runner-argv")"
confirmed_dry_argv="$(sed -n '2p' "$operator_fixture/planning/artifacts/runs/runner-argv")"
live_argv="$(sed -n '3p' "$operator_fixture/planning/artifacts/runs/runner-argv")"
assert_contains "operator dry-run flag" "$dry_argv" "--dry-run"
assert_contains "operator confirmed dry-run flag" "$confirmed_dry_argv" "--dry-run"
assert_not_contains "operator live dry-run" "$live_argv" "--dry-run"
[[ "$(wc -l <"$operator_fixture/planning/artifacts/runs/freshness")" == "3" ]] || \
  fail "operator did not check binary freshness before dry-run, recheck, and live invocation"

refused_before="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
refused_input="$(printf '\n\n\n1\n1\n\nCI,Planning,CodeQL\n1\n0\nno\n')"
printf '%s\n' "$refused_input" | PATH="$operator_fixture/fake-bin:$PATH" \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" \
  AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  "$operator_fixture/scripts/autonomous-loop/operator.sh" >/dev/null 2>&1
refused_rc=$?
[[ $refused_rc -ne 0 ]] || fail "operator confirmation refusal unexpectedly succeeded"
refused_after="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
[[ $((refused_after - refused_before)) -eq 1 ]] || fail "operator refusal invoked the live runner"

missing_before="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
printf '\n' | PATH="$operator_fixture/fake-bin:$PATH" AUTONOMOUS_LOOP_ROOT="$operator_fixture" \
  AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  "$operator_fixture/scripts/autonomous-loop/operator.sh" >/dev/null 2>&1
missing_rc=$?
[[ $missing_rc -ne 0 ]] || fail "operator accepted incomplete choices"
missing_after="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
[[ "$missing_before" == "$missing_after" ]] || fail "incomplete choices invoked the runner"

drift_before="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
drift_input="$(printf '\n\n\n1\n1\n\nCI,Planning,CodeQL\n1\n0\nRUN\n')"
drift_output="$(printf '%s\n' "$drift_input" | PATH="$operator_fixture/fake-bin:$PATH" \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  AUTONOMOUS_OPERATOR_DRIFT_DRY=1 "$operator_fixture/scripts/autonomous-loop/operator.sh" 2>&1)"
drift_rc=$?
[[ $drift_rc -ne 0 ]] || fail "changed confirmed dry-run unexpectedly launched"
assert_contains "changed confirmed dry-run" "$drift_output" "dry-run changed after confirmation"
drift_after="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
[[ $((drift_after - drift_before)) -eq 2 ]] || fail "changed dry-run invoked a live runner"
rm -f "$operator_fixture/planning/artifacts/runs/dry-count"

touch "$operator_fixture/planning/artifacts/runs/freshness-refusal"
cat >"$operator_fixture/fake-bin/task" <<'EOF'
#!/usr/bin/env bash
exit 9
EOF
tool_before="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
tool_input="$(printf '\n\n\n1\n1\n\nCI,Planning,CodeQL\n1\n0\n')"
printf '%s\n' "$tool_input" | PATH="$operator_fixture/fake-bin:$PATH" AUTONOMOUS_LOOP_ROOT="$operator_fixture" \
  AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  "$operator_fixture/scripts/autonomous-loop/operator.sh" >/dev/null 2>&1
tool_rc=$?
[[ $tool_rc -ne 0 ]] || fail "operator accepted failed binary freshness"
tool_after="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
[[ "$tool_before" == "$tool_after" ]] || fail "binary freshness refusal invoked the runner"
cat >"$operator_fixture/fake-bin/task" <<'EOF'
#!/usr/bin/env bash
[[ "$*" == "taskrail:check" ]] || exit 2
printf '%s\n' checked >>"$AUTONOMOUS_OPERATOR_CAPTURE/freshness"
EOF
chmod +x "$operator_fixture/fake-bin/task"

partial_log="$operator_fixture/planning/artifacts/runs/partial.log"
printf '%s\n' \
  '[autonomous-loop] worker launched: T-900 (pid 100)' \
  '[autonomous-loop] worker launched: T-901 (pid 101)' \
  '[autonomous-loop] worker failed: T-900 (rc 7)' \
  '[autonomous-loop] worker done: T-901 (completed_pass)' \
  '[autonomous-loop] integration child start: T-901' \
  '[autonomous-loop] integration child done: T-901' \
  '[autonomous-loop] integrated: T-901' \
  '[autonomous-loop] unpublished: T-900 - worker failed' \
  '[autonomous-loop] STOP: batch gate failed over the final integration head' >"$partial_log"
partial_output="$(AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 AUTONOMOUS_LOOP_ROOT="$operator_fixture" bash -c \
  'source "$1"; summarize_runner "$2"' _ "$OPERATOR" "$partial_log" 2>&1)"
assert_contains "partial ranked failure" "$partial_output" "worker[1]: T-900 - failed"
assert_contains "partial sibling success" "$partial_output" "worker[2]: T-901 - completed_pass"
assert_contains "partial unpublished" "$partial_output" "integration: T-900 - worker failed - unpublished"
assert_contains "conflict child" "$partial_output" "integration-conflict: T-901 - child completed"
assert_contains "failed aggregate gate" "$partial_output" "local-gate: fail"

settle_script="$TMP_ROOT/operator-settle.sh"
settle_marker="$TMP_ROOT/operator-settled"
cat >"$settle_script" <<'EOF'
#!/usr/bin/env bash
sleep 1
printf '%s\n' settled >"$AUTONOMOUS_SETTLE_MARKER"
EOF
chmod +x "$settle_script"
AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 AUTONOMOUS_LOOP_ROOT="$operator_fixture" \
  AUTONOMOUS_SETTLE_MARKER="$settle_marker" bash -c \
  'source "$1"; run_supervised "$2" "$3"; [[ -z "$OBSERVED_INTERRUPT" ]]' \
  _ "$OPERATOR" "$TMP_ROOT/operator-signal.log" "$settle_script" >/dev/null 2>&1 &
signal_parent=$!
sleep 0.1
kill -TERM "$signal_parent"
wait "$signal_parent"
signal_rc=$?
[[ $signal_rc -ne 0 ]] || fail "operator interruption was reported as success"
[[ -f "$settle_marker" ]] || fail "operator interruption stopped the existing runner"

failing_tee="$TMP_ROOT/operator-failing-tee"
cat >"$failing_tee" <<'EOF'
#!/usr/bin/env bash
while IFS= read -r _; do :; done
exit 7
EOF
chmod +x "$failing_tee"
diagnostic_output="$(AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 AUTONOMOUS_LOOP_ROOT="$operator_fixture" \
  AUTONOMOUS_OPERATOR_TEE="$failing_tee" bash -c \
  'source "$1"; run_supervised "$2" bash -c "printf output"; printf "diagnostics=%s\n" "$DIAGNOSTIC_PRESERVATION"' \
  _ "$OPERATOR" "$TMP_ROOT/operator-tee-failure.log" 2>&1)"
assert_contains "diagnostic preservation failure" "$diagnostic_output" "diagnostics=fail"

xdg_root="$TMP_ROOT/operator-xdg"
repo_hash="$(printf '%s' "$operator_fixture" | sha256sum | awk '{print $1}')"
recovery_root="$xdg_root/taskrail/autonomous-loop/$repo_hash"
bundle="$recovery_root/fixture-bundle"
mkdir -p "$bundle"
chmod 700 "$xdg_root" "$xdg_root/taskrail" "$xdg_root/taskrail/autonomous-loop" "$recovery_root" "$bundle"
base_head="$(git -C "$operator_fixture" rev-parse HEAD)"
base_index="$(git -C "$operator_fixture" write-tree)"
printf '%s\n' 'fixture candidate' >"$operator_fixture/README.md"
printf '%s\n' 'last_verification_result: pass for T-900 at 2026-08-20T00:00:00Z (verification_id=0123456789abcdef0123456789abcdef)' >"$operator_fixture/planning/STATE.md"
candidate_index="$TMP_ROOT/operator-candidate-index"
cp "$(git -C "$operator_fixture" rev-parse --git-path index)" "$candidate_index"
GIT_INDEX_FILE="$candidate_index" git -C "$operator_fixture" add -A
candidate_tree="$(GIT_INDEX_FILE="$candidate_index" git -C "$operator_fixture" write-tree)"
rm -f "$candidate_index"
report_path=planning/artifacts/verify/T-900/fixture/report.json
mkdir -p "$operator_fixture/planning/artifacts/verify/T-900/fixture"
printf '%s\n' '{"schema_version":1,"task_id":"T-900","task_title":"Fixture","result":"pass","verification_id":"0123456789abcdef0123456789abcdef","summary":"fixture pass","generated_at":"2026-08-20T00:00:00Z","spec_ref":"specs/v0.5.0.md#fixture","artifacts":[]}' >"$operator_fixture/$report_path"
printf '%s\n' fixture-log >"$operator_fixture/planning/artifacts/runs/fixture.log"
printf '%s\n' 'test: recover fixture (T-900)' >"$bundle/commit-message"
printf '%s\n' 1 >"$bundle/schema_version"
printf '%s\n' "$operator_fixture" >"$bundle/repository"
printf '%s\n' T-900 >"$bundle/task_id"
printf '%s\n' completed_pass >"$bundle/outcome"
printf '%s\n' "$base_head" >"$bundle/base_head"
printf '%s\n' "$base_head" >"$bundle/base_remote"
printf '%s\n' "$base_index" >"$bundle/base_index"
printf '%s\n' "$candidate_tree" >"$bundle/candidate_tree"
printf '%s\n' "$report_path" >"$bundle/report_path"
printf '%s\n' "$(sha256sum "$operator_fixture/$report_path" | awk '{print $1}')" >"$bundle/report_sha256"
printf '%s\n' 2026-08-20T00:00:00Z >"$bundle/generated_at"
printf '%s\n' "$(sha256sum "$operator_fixture/scripts/autonomous-loop/queue.tsv" | awk '{print $1}')" >"$bundle/queue_sha256"
printf '%s\n' "$(sha256sum "$bundle/commit-message" | awk '{print $1}')" >"$bundle/commit_message_sha256"
: >"$bundle/existing-reports"
printf 'planning/tasks/T-900.md\t%s\n' "$(printf selected | sha256sum | awk '{print $1}')" >"$bundle/task-manifest"
printf '%s\n' planning/artifacts/runs/fixture.log >"$bundle/run_log"
printf '%s\n' 2h >"$bundle/timeout"
printf '%s\n' 2026-08-20T00:00:00Z >"$bundle/created_at"
printf '%s\n' complete >"$bundle/COMPLETE"
chmod 600 "$bundle"/*

bundle_output="$(PATH="$operator_fixture/fake-bin:$PATH" AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" XDG_STATE_HOME="$xdg_root" bash -c \
  'source "$1"; inspect_recovery_bundle "$2"; printf "result=%s\n" "$RECOVERY_STATUS"' \
  _ "$OPERATOR" "$bundle" 2>&1)"
assert_contains "valid recovery bundle" "$bundle_output" "result=undelivered"
assert_contains "recovery candidate identity" "$bundle_output" "exact HEAD/base/index/remote/candidate bytes match"

for invalid in incomplete wrong-repository wrong-task stale-base tampered-message stale-report stale-manifest; do
  invalid_bundle="$recovery_root/$invalid"
  cp -a "$bundle" "$invalid_bundle"
  case "$invalid" in
    incomplete) rm -f "$invalid_bundle/COMPLETE" ;;
    wrong-repository) printf '%s\n' /tmp/wrong-repository >"$invalid_bundle/repository" ;;
    wrong-task) printf '%s\n' T-999 >"$invalid_bundle/task_id" ;;
    stale-base) printf '%040d\n' 0 >"$invalid_bundle/base_head" ;;
    tampered-message) printf '%s\n' tampered >>"$invalid_bundle/commit-message" ;;
    stale-report) printf '%s\n' 2026-08-21T00:00:00Z >"$invalid_bundle/generated_at" ;;
    stale-manifest) printf 'planning/tasks/T-901.md\t%s\n' "$(printf stale | sha256sum | awk '{print $1}')" >>"$invalid_bundle/task-manifest" ;;
  esac
  chmod 600 "$invalid_bundle"/*
  PATH="$operator_fixture/fake-bin:$PATH" AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 \
    AUTONOMOUS_LOOP_ROOT="$operator_fixture" XDG_STATE_HOME="$xdg_root" bash -c \
    'source "$1"; inspect_recovery_bundle "$2"' _ "$OPERATOR" "$invalid_bundle" >/dev/null 2>&1 \
    && fail "$invalid recovery bundle was accepted"
done

printf '%s\n' dirty >"$operator_fixture/unrelated.txt"
PATH="$operator_fixture/fake-bin:$PATH" AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" XDG_STATE_HOME="$xdg_root" bash -c \
  'source "$1"; inspect_recovery_bundle "$2"' _ "$OPERATOR" "$bundle" >/dev/null 2>&1 \
  && fail "dirty recovery source was accepted"
rm -f "$operator_fixture/unrelated.txt"

recovery_before="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
recovery_input="$(printf '\n\n\n1\n1\n\nCI,Planning,CodeQL\n1\n0\nRUN\nno\nno\n')"
recovery_output="$(printf '%s\n' "$recovery_input" | PATH="$operator_fixture/fake-bin:$PATH" \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  AUTONOMOUS_OPERATOR_BUNDLE="$bundle" XDG_STATE_HOME="$xdg_root" \
  "$operator_fixture/scripts/autonomous-loop/operator.sh" 2>&1)"
assert_contains "recovery refusal" "$recovery_output" "delivery recovery refused; it will not be retried"
[[ ! -e "$operator_fixture/planning/artifacts/runs/resume-invocations" ]] || fail "refused recovery invoked resume"
recovery_after="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
[[ $((recovery_after - recovery_before)) -eq 3 ]] || fail "recovery refusal repeated a runner invocation"

resume_input="$(printf '\n\n\n1\n1\n\nCI,Planning,CodeQL\n1\n0\nRUN\nRESUME-DELIVERY\nno\n')"
resume_output="$(printf '%s\n' "$resume_input" | PATH="$operator_fixture/fake-bin:$PATH" \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  AUTONOMOUS_OPERATOR_BUNDLE="$bundle" XDG_STATE_HOME="$xdg_root" \
  "$operator_fixture/scripts/autonomous-loop/operator.sh" 2>&1)"
assert_contains "recovery resume exact command" "$resume_output" "--resume-delivery"
assert_contains "recovery resume delivery" "$resume_output" "delivery: pass (delivery-only recovery)"
[[ "$(wc -l <"$operator_fixture/planning/artifacts/runs/resume-invocations")" == 1 ]] || \
  fail "delivery recovery did not invoke exactly once"

quota_before="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
quota_input="$(printf '\n\n\n2\n2\n\n\n\nCI,Planning,CodeQL\n1\n0\nRUN\nyes\nprovider stderr\nreset at 09:00 UTC\n')"
quota_output="$(printf '%s\n' "$quota_input" | PATH="$operator_fixture/fake-bin:$PATH" \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  AUTONOMOUS_OPERATOR_QUOTA_BATCH=1 \
  "$operator_fixture/scripts/autonomous-loop/operator.sh" 2>&1)"
assert_contains "quota attribution" "$quota_output" "attributed external quota evidence from provider stderr"
assert_contains "quota sibling drain" "$quota_output" "worker[2]: T-901 - completed_pass"
assert_contains "quota ordinary integration" "$quota_output" "integration: T-901 - integrated"
assert_contains "quota ordinary unpublished" "$quota_output" "integration: T-900 - provider exited 7 - unpublished"
assert_contains "quota accounting" "$quota_output" "configured cap iterations=2 parallel=2; launched=2 ids=T-900,T-901"
assert_contains "quota no refund" "$quota_output" "unused capacity is not an attempt, refund, or carry-forward"
assert_contains "quota fresh invocation" "$quota_output" "fresh preflight, exact dry-run, newly explicit finite budget, and fresh confirmation"
assert_contains "quota no scheduling" "$quota_output" "No background wake-up"
quota_after="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
[[ $((quota_after - quota_before)) -eq 3 ]] || fail "quota handling automatically relaunched the runner"

sequential_quota_before="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
sequential_quota_input="$(printf '\n\n\n2\n1\n\nCI,Planning,CodeQL\n1\n0\nRUN\nyes\nprovider stderr\nreset tomorrow morning\n')"
sequential_quota_output="$(printf '%s\n' "$sequential_quota_input" | PATH="$operator_fixture/fake-bin:$PATH" \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" AUTONOMOUS_OPERATOR_CAPTURE="$operator_fixture/planning/artifacts/runs" \
  AUTONOMOUS_OPERATOR_QUOTA_SEQUENTIAL=1 "$operator_fixture/scripts/autonomous-loop/operator.sh" 2>&1)"
assert_contains "sequential quota attempt count" "$sequential_quota_output" "configured cap iterations=2 parallel=1; launched=1 ids=T-900"
assert_contains "timezone-free reset attribution" "$sequential_quota_output" "attributed external quota evidence from provider stderr: reset tomorrow morning"
assert_not_contains "timezone-free reset invention" "$sequential_quota_output" "2026-"
sequential_quota_after="$(wc -l <"$operator_fixture/planning/artifacts/runs/runner-argv")"
[[ $((sequential_quota_after - sequential_quota_before)) -eq 3 ]] || fail "sequential quota handling relaunched the runner"

delivered_head="$(git -C "$operator_fixture" rev-parse HEAD)"
bundle_output="$(PATH="$operator_fixture/fake-bin:$PATH" AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 \
  AUTONOMOUS_LOOP_ROOT="$operator_fixture" XDG_STATE_HOME="$xdg_root" bash -c \
  'source "$1"; inspect_recovery_bundle "$2"; printf "result=%s\n" "$RECOVERY_STATUS"' \
  _ "$OPERATOR" "$bundle" 2>&1)"
assert_contains "delivered recovery bundle" "$bundle_output" "result=delivered"
assert_contains "delivered recovery identity" "$bundle_output" "delivered commit/tree/message and local main/HEAD/origin agree"

ci_fixture="$TMP_ROOT/operator-ci"
mkdir -p "$ci_fixture/bin" "$ci_fixture/state"
cat >"$ci_fixture/bin/gh" <<'EOF'
#!/usr/bin/env bash
count_file="$AUTONOMOUS_CI_STATE/count"
count=0
[[ ! -f "$count_file" ]] || count="$(<"$count_file")"
count=$((count + 1))
printf '%s\n' "$count" >"$count_file"
case "$AUTONOMOUS_CI_SCENARIO" in
  delayed)
    ((count > 1)) || exit 0
    printf 'CI\t%s\tcompleted\tsuccess\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\n' "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD"
    ;;
  pending-pass)
    if ((count == 1)); then status=in_progress; conclusion=; else status=completed; conclusion=success; fi
    printf 'CI\t%s\t%s\t%s\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\n' "$AUTONOMOUS_CI_HEAD" "$status" "$conclusion" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD"
    ;;
  late)
    if ((count == 1)); then
      printf 'CI\t%s\tcompleted\tsuccess\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\n' "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD"
    elif ((count == 2)); then
      printf 'CI\t%s\tcompleted\tsuccess\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\nLate workflow\t%s\tin_progress\t\t4\n' "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD"
    else
      printf 'CI\t%s\tcompleted\tsuccess\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\nLate workflow\t%s\tcompleted\tfailure\t4\n' "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD"
    fi
    ;;
  fail) printf 'CI\t%s\tcompleted\tfailure\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\n' "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" ;;
  cancelled) printf 'CI\t%s\tcompleted\tcancelled\t1\nPlanning\t%s\tcompleted\tsuccess\t2\nCodeQL\t%s\tcompleted\tsuccess\t3\n' "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" "$AUTONOMOUS_CI_HEAD" ;;
  unrelated) printf 'CI\t%s\tcompleted\tsuccess\t1\n' 0000000000000000000000000000000000000000 ;;
esac
EOF
chmod +x "$ci_fixture/bin/gh"

for scenario in delayed pending-pass late fail cancelled unrelated; do
  rm -f "$ci_fixture/state/count"
  ci_output="$(PATH="$ci_fixture/bin:$PATH" AUTONOMOUS_LOOP_OPERATOR_LIBRARY=1 \
    AUTONOMOUS_CI_STATE="$ci_fixture/state" AUTONOMOUS_CI_SCENARIO="$scenario" \
    AUTONOMOUS_CI_HEAD=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
    AUTONOMOUS_LOOP_ROOT="$operator_fixture" bash -c \
    'source "$1"; observe_remote_ci "$AUTONOMOUS_CI_HEAD" "CI,Planning,CodeQL" 2 0; printf "result=%s\n" "$REMOTE_CI_STATUS"' \
    _ "$OPERATOR" 2>&1)"
  case "$scenario" in
    delayed | pending-pass) assert_contains "CI $scenario" "$ci_output" "result=pass" ;;
    late | fail) assert_contains "CI $scenario" "$ci_output" "result=fail" ;;
    cancelled) assert_contains "CI cancelled" "$ci_output" "result=cancelled" ;;
    unrelated) assert_contains "CI exact-head missing" "$ci_output" "result=missing" ;;
  esac
done
