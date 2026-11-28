package toolchain_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// indentOf counts leading spaces; the workflow parsers below key off YAML
// indentation instead of a full parse, like the rest of this package.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// yamlBlock returns the lines nested under the first line at exactly indent
// whose trimmed text is header, stopping at the next non-blank, non-comment
// line indented at or above it.
func yamlBlock(lines []string, indent int, header string) []string {
	start := -1
	for i, line := range lines {
		if indentOf(line) == indent && strings.TrimSpace(line) == header {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	var body []string
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && indentOf(line) <= indent {
			break
		}
		body = append(body, line)
	}
	return body
}

// workflowJobs returns each job's body lines keyed by job id, in file order.
func workflowJobs(content string) (ids []string, jobs map[string][]string) {
	body := yamlBlock(strings.Split(content, "\n"), 0, "jobs:")
	jobs = map[string][]string{}
	current := ""
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if indentOf(line) == 2 && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "#") {
			current = strings.TrimSuffix(trimmed, ":")
			ids = append(ids, current)
			continue
		}
		if current != "" {
			jobs[current] = append(jobs[current], line)
		}
	}
	return ids, jobs
}

var taskRunStep = regexp.MustCompile(`^run:\s*task\s+(\S+)\s*$`)

// jobTaskSteps returns the target of every single-command `run: task <name>`
// step in a job, in order. Steps carrying an `if:` are skipped when
// skipConditional is set, since they run on only some matrix legs.
func jobTaskSteps(job []string, skipConditional bool) []string {
	var targets []string
	for _, block := range workflowStepBlocks(strings.Join(job, "\n")) {
		target, conditional := "", false
		for _, line := range block {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "if:") {
				conditional = true
			}
			if m := taskRunStep.FindStringSubmatch(trimmed); m != nil {
				target = m[1]
			}
		}
		if target != "" && (!conditional || !skipConditional) {
			targets = append(targets, target)
		}
	}
	return targets
}

// taskfileTaskCalls returns the `- task: <name>` entries of one Taskfile task.
func taskfileTaskCalls(t *testing.T, header string) []string {
	t.Helper()
	block := taskfileBlock(readFile(t, repoRoot(t), "Taskfile.yml"), header)
	if block == nil {
		t.Fatalf("Taskfile.yml must define %s", header)
	}
	var calls []string
	for _, line := range block {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "- task:"); ok {
			calls = append(calls, strings.TrimSpace(name))
		}
	}
	return calls
}

// appendNew appends the names not already present, keeping first-seen order.
func appendNew(list []string, names ...string) []string {
	for _, name := range names {
		if !slices.Contains(list, name) {
			list = append(list, name)
		}
	}
	return list
}

func TestJobTaskStepsSkipsConditionalAndNonTaskSteps(t *testing.T) {
	job := strings.Split(""+
		"    steps:\n"+
		"      - name: A\n"+
		"        run: task build\n"+
		"      - name: B\n"+
		"        if: matrix.runner == 'windows-latest'\n"+
		"        run: task taskrail:check\n"+
		"      - name: C\n"+
		"        run: taskrail validate\n", "\n")
	if got, want := jobTaskSteps(job, true), []string{"build"}; !slices.Equal(got, want) {
		t.Errorf("jobTaskSteps(skip) = %v, want %v", got, want)
	}
	if got, want := jobTaskSteps(job, false), []string{"build", "taskrail:check"}; !slices.Equal(got, want) {
		t.Errorf("jobTaskSteps(all) = %v, want %v", got, want)
	}
}

// `task check` is the one local command for "would CI pass". It only stays
// trustworthy if it runs exactly what CI runs: a step added to CI and not here
// lets a contributor push a red build after a green local run.
func TestLocalCheckMirrorsTheCIJobs(t *testing.T) {
	_, jobs := workflowJobs(readFile(t, repoRoot(t), ".github/workflows/ci.yml"))
	for _, id := range []string{"checks", "build-test", "cross-compile"} {
		if jobs[id] == nil {
			t.Fatalf("ci.yml has no %s job", id)
		}
	}
	var want []string
	want = appendNew(want, jobTaskSteps(jobs["checks"], false)...)
	want = appendNew(want, jobTaskSteps(jobs["build-test"], true)...)
	want = appendNew(want, jobTaskSteps(jobs["cross-compile"], true)...)

	if got := taskfileTaskCalls(t, "check:"); !slices.Equal(got, want) {
		t.Errorf("Taskfile `check` must run the ci.yml task steps in order\n got: %v\nwant: %v", got, want)
	}
}

// Same contract for the doc/planning fast lane, which is the only CI a
// planning-only change ever gets.
func TestLocalPlanningCheckMirrorsThePlanningWorkflow(t *testing.T) {
	ids, jobs := workflowJobs(readFile(t, repoRoot(t), ".github/workflows/planning.yml"))
	if len(ids) == 0 {
		t.Fatal("planning.yml declares no jobs")
	}
	var want []string
	for _, id := range ids {
		want = appendNew(want, jobTaskSteps(jobs[id], false)...)
	}
	if got := taskfileTaskCalls(t, "check:planning:"); !slices.Equal(got, want) {
		t.Errorf("Taskfile `check:planning` must run the planning.yml task steps in order\n got: %v\nwant: %v", got, want)
	}
}

// triggerPaths returns the quoted entries of on.<trigger>.<key>, or nil.
func triggerPaths(content, trigger, key string) []string {
	on := yamlBlock(strings.Split(content, "\n"), 0, "on:")
	list := yamlBlock(yamlBlock(on, 2, trigger+":"), 4, key+":")
	var paths []string
	for _, line := range list {
		if entry, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok {
			paths = append(paths, strings.Trim(entry, `"'`))
		}
	}
	return paths
}

// A path in ci.yml's paths-ignore but not in planning.yml's paths is a change
// no workflow checks at all; the reverse runs the fast lane for nothing. The
// two lists are one decision written twice, so they must match exactly.
func TestWorkflowPathLanesAreExactMirrors(t *testing.T) {
	root := repoRoot(t)
	ci := readFile(t, root, ".github/workflows/ci.yml")
	planning := readFile(t, root, ".github/workflows/planning.yml")

	reference := triggerPaths(ci, "pull_request", "paths-ignore")
	if len(reference) == 0 {
		t.Fatal("ci.yml pull_request must declare paths-ignore")
	}
	for _, lane := range []struct{ file, content, trigger, key string }{
		{"ci.yml", ci, "push", "paths-ignore"},
		{"planning.yml", planning, "pull_request", "paths"},
		{"planning.yml", planning, "push", "paths"},
	} {
		got := triggerPaths(lane.content, lane.trigger, lane.key)
		if !slices.Equal(got, reference) {
			t.Errorf("%s on.%s.%s = %v, want ci.yml on.pull_request.paths-ignore %v",
				lane.file, lane.trigger, lane.key, got, reference)
		}
	}
}

// jobTimeout returns the job-level timeout-minutes, or 0 when absent.
func jobTimeout(job []string) int {
	for _, line := range job {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "timeout-minutes:"); ok && indentOf(line) == 4 {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

// GitHub's default job timeout is six hours, so a hung step otherwise holds a
// runner, and the PR's required check, for that long.
func TestEveryJobDeclaresATimeout(t *testing.T) {
	for _, file := range workflowFiles(t) {
		ids, jobs := workflowJobs(file.content)
		if len(ids) == 0 {
			t.Errorf("%s declares no jobs", file.name)
		}
		for _, id := range ids {
			if jobTimeout(jobs[id]) <= 0 {
				t.Errorf("%s job %s must declare a positive job-level timeout-minutes", file.name, id)
			}
		}
	}

	// `go test -timeout 60m` bounds each test binary, not the step, so a green
	// native Windows job has taken 116 minutes. The job ceiling only exists to
	// stop a hung runner; set below that, it cancels healthy runs.
	_, jobs := workflowJobs(readFile(t, repoRoot(t), ".github/workflows/ci.yml"))
	if got := jobTimeout(jobs["build-test"]); got < 120 {
		t.Errorf("ci.yml build-test timeout-minutes = %d; must be at least 120 (green Windows jobs have taken 116)", got)
	}
}

// The local pre-push guard is opt-in and skipped by --no-verify, so the
// server-side check is the one that holds. A path filter would let a doc-only
// push carry an attributed commit past it.
func TestCommitPolicyWorkflowChecksEveryChange(t *testing.T) {
	content := readFile(t, repoRoot(t), ".github/workflows/commit-policy.yml")
	for _, trigger := range []string{"pull_request", "push"} {
		for _, key := range []string{"paths", "paths-ignore"} {
			if triggerPaths(content, trigger, key) != nil {
				t.Errorf("commit-policy.yml on.%s must not filter by %s", trigger, key)
			}
		}
	}
	for _, want := range []string{"fetch-depth: 0", "bash scripts/check-push-messages.sh \"$range\""} {
		if !strings.Contains(content, want) {
			t.Errorf("commit-policy.yml must contain %q", want)
		}
	}
}
