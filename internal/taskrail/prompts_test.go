package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRenderPromptValidatesAndSubstitutesDeclaredTokens(t *testing.T) {
	const template = "Task {{TASK_ID}} at {{TASK_PATH}}; braces { } stay literal.\n"
	result, err := RenderPrompt(PromptRenderInput{
		Template:       []byte(template),
		DeclaredTokens: []string{"TASK_ID", "TASK_PATH"},
		Values: map[string]string{
			"TASK_ID":   "T-250",
			"TASK_PATH": "planning/tasks/T-250.md",
		},
	})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}
	if result.Content != "Task T-250 at planning/tasks/T-250.md; braces { } stay literal.\n" {
		t.Fatalf("rendered content = %q", result.Content)
	}
	if result.TemplateSHA256 != "0ee68fcf3aa29dcdaf40dd37ad69939f203409380b8d9e0368ee858bcbbc5b9b" || result.SHA256 != "fb94fd45f4235eec1fe01621a21bef252e0f6f353ca1a586110b6fe543d3c5bd" {
		t.Fatalf("rendered digests = %+v, want exact-byte goldens", result)
	}

	for _, test := range []struct {
		name  string
		input PromptRenderInput
	}{
		{"unknown token", PromptRenderInput{Template: []byte("{{OTHER}}"), DeclaredTokens: []string{"KNOWN"}, Values: map[string]string{"KNOWN": "value"}}},
		{"unresolved token", PromptRenderInput{Template: []byte("{{KNOWN}}"), DeclaredTokens: []string{"KNOWN"}, Values: map[string]string{}}},
		{"unknown value", PromptRenderInput{Template: []byte("plain"), Values: map[string]string{"OTHER": "value"}}},
		{"duplicate declaration", PromptRenderInput{Template: []byte("plain"), DeclaredTokens: []string{"KNOWN", "KNOWN"}, Values: map[string]string{"KNOWN": "value"}}},
		{"invalid declaration", PromptRenderInput{Template: []byte("plain"), DeclaredTokens: []string{"known"}, Values: map[string]string{"known": "value"}}},
		{"malformed lower case token", PromptRenderInput{Template: []byte("{{known}}"), Values: map[string]string{}}},
		{"malformed leading digit token", PromptRenderInput{Template: []byte("{{0KNOWN}}"), Values: map[string]string{}}},
		{"malformed leading underscore token", PromptRenderInput{Template: []byte("{{_KNOWN}}"), Values: map[string]string{}}},
		{"malformed punctuation token", PromptRenderInput{Template: []byte("{{KNOWN-NAME}}"), Values: map[string]string{}}},
		{"malformed whitespace token", PromptRenderInput{Template: []byte("{{KNOWN NAME}}"), Values: map[string]string{}}},
		{"malformed UTF-8 token", PromptRenderInput{Template: []byte("{{KNOWN\u00c9}}"), Values: map[string]string{}}},
		{"empty token", PromptRenderInput{Template: []byte("{{}}"), Values: map[string]string{}}},
		{"unterminated token", PromptRenderInput{Template: []byte("{{KNOWN"), Values: map[string]string{}}},
		{"invalid UTF-8 template", PromptRenderInput{Template: []byte{0xff}, Values: map[string]string{}}},
		{"BOM template", PromptRenderInput{Template: append([]byte{0xef, 0xbb, 0xbf}, []byte("plain")...), Values: map[string]string{}}},
		{"oversize template", PromptRenderInput{Template: []byte(strings.Repeat("x", promptReplacementLimit+1)), Values: map[string]string{}}},
		{"invalid UTF-8 value", PromptRenderInput{Template: []byte("{{KNOWN}}"), DeclaredTokens: []string{"KNOWN"}, Values: map[string]string{"KNOWN": string([]byte{0xff})}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := RenderPrompt(test.input); err == nil {
				t.Fatal("render accepted invalid prompt input")
			}
		})
	}

	if _, err := RenderPrompt(PromptRenderInput{Template: []byte(strings.Repeat("x", promptReplacementLimit))}); err != nil {
		t.Fatalf("render size-limit template: %v", err)
	}
}

func TestTaskAuthoringPromptDefinesOutcomeFocusedBodyContract(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-author", "Author", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))

	result, err := svc.PromptRender(PromptRenderCommandInput{ID: "task-authoring", Task: "T-001-author"})
	if err != nil {
		t.Fatalf("render task authoring prompt: %v", err)
	}
	content := strings.Join(strings.Fields(result.Content), " ")
	for _, fixture := range []struct {
		name string
		want string
	}{
		{"managed reads", "Use `taskrail task show T-001-author` and `taskrail spec show v0.1.0` to inspect managed bytes; do not read logical paths directly."},
		{"aligned", "one independently meaningful user, operator, or system outcome and the invariant it establishes"},
		{"oversized", "Split an oversized proposal when independently useful parts have separate acceptance or durable oracles"},
		{"fragmented", "Merge a fragmented proposal when code, tests, documentation, migration, and cross-layer changes together establish one observable result"},
		{"integration owner", "Name which resulting task owns required integrated behavior."},
		{"mechanical size proxy", "Do not use file count, criterion count, implementation layers, or estimates as size proxies."},
		{"relevant boundaries", "For every acceptance criterion, map relevant actor, precondition, state, action, and expected success; include failure and boundary observations where they materially differ."},
		{"evidence layer", "cheapest sufficient evidence layer"},
		{"durable oracle", "public or durable oracle"},
		{"shallow oracle", "a shallow oracle when it does not establish persisted or user-visible behavior"},
		{"regression", "regression-sensitive evidence"},
		{"manual evidence", "Manual probes must be reproducible and sandbox-first, name prerequisites and cleanup, and distinguish expected behavior from observed evidence."},
		{"non-code evidence", "Non-code tasks must name an equivalent inspectable check or explain why a test category is not applicable."},
		{"reuse rationale", "Before proposing a new abstraction, inventory relevant existing repository primitives and record why extension or reuse is insufficient."},
		{"non-todo", "Refuse to author a task whose status is not exactly `todo`."},
		{"over-prescribed", "Refuse vague suite-pass evidence, unnecessary internal prescription, speculative checklist scope, and direct task mutation."},
		{"read-only", "Do not mutate the task, its status, dependencies, spec, or repository files."},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if !strings.Contains(content, fixture.want) {
				t.Errorf("task-authoring prompt missing %q:\n%s", fixture.want, result.Content)
			}
		})
	}
}

func TestDecompositionPromptsDefineReviewedOutcomeContract(t *testing.T) {
	common := []string{
		"one independently meaningful user, operator, or system outcome",
		"Split independently useful outcomes",
		"Do not split one outcome by file, layer, discipline, phase, or estimate",
		"integrated behavior",
		"public or durable oracle",
		"shallow oracle",
		"failure and boundary",
		"operator gates",
		"real specification heading",
		"real dependencies",
		"omit `loop_policy` and `loop_reason`",
		"implicitly held",
		"exact SHA-256",
		"fresh context",
		"Do not mutate",
	}
	for _, id := range []string{"task-decomposition", "task-decomposition-adversarial"} {
		content := strings.Join(strings.Fields(string(builtinPromptTemplate(t, id))), " ")
		for _, want := range common {
			if !strings.Contains(content, want) {
				t.Errorf("%s missing %q:\n%s", id, want, content)
			}
		}
		if !strings.Contains(content, "## Description") || !strings.Contains(content, "## Acceptance") || !strings.Contains(content, "## Verification Notes") {
			t.Errorf("%s lacks strict body headings", id)
		}
	}
}

func TestDecompositionPromptsDefineCompleteReviewedSession(t *testing.T) {
	author := strings.Join(strings.Fields(string(builtinPromptTemplate(t, "task-decomposition"))), " ")
	for _, want := range []string{
		"final post-spec `manifest.json`",
		"active spec, use `coverage --json`",
		"inactive spec, enumerate live anchors",
		"Every normative requirement needs one exact quote or valid line range",
	} {
		if !strings.Contains(author, want) {
			t.Errorf("task-decomposition missing %q:\n%s", want, author)
		}
	}

	reviewer := strings.Join(strings.Fields(string(builtinPromptTemplate(t, "task-decomposition-adversarial"))), " ")
	for _, want := range []string{
		"fresh review process or fresh agent context",
		"If prompt resolution changes before publication, abandon the session",
	} {
		if !strings.Contains(reviewer, want) {
			t.Errorf("task-decomposition-adversarial missing %q:\n%s", want, reviewer)
		}
	}
}

func TestTaskImplementationPromptDefinesLifecycleCompleteWorkflow(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-implement", "Implement", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))

	result, err := svc.PromptRender(PromptRenderCommandInput{ID: "task-implementation", Task: "T-001-implement"})
	if err != nil {
		t.Fatalf("render task implementation prompt: %v", err)
	}
	content := strings.Join(strings.Fields(result.Content), " ")
	for _, fixture := range []struct {
		name string
		want string
	}{
		{"managed reads", "Use `taskrail task show T-001-implement --json` and `taskrail spec show v0.1.0 --json` to inspect managed bytes; do not read logical paths directly."},
		{"sizing", "Before `start`, apply the outcome-focused sizing rubric: require one independently meaningful outcome, a bounded observable result, explicit dependencies and operator gates, and clear integrated-behavior ownership."},
		{"replan", "Stop for reviewed decomposition or clarification rather than implement an arbitrary slice when the task bundles independently useful outcomes, fragments one outcome, or leaves integration ownership unclear."},
		{"invariants", "Trace acceptance requirements to observable executable or inspectable evidence before implementation."},
		{"writer freshness", "Before every Taskrail state writer, run the source-checkout freshness guard when the repository provides one; stop and apply its named remedy if it fails. Pass `--json` to every consumed Taskrail command, parse its common result envelope, and check command and writer exits."},
		{"tdd", "Implement the smallest safe change; begin behavior changes with a failing test whenever practical."},
		{"simplification", "Inspect the verified implementation for unnecessary complexity and simplify only when behavior is preserved; independent simplification delegation is optional."},
		{"reviewer", "Freeze the verified implementation, then run one broad implementation-review round with one fresh reviewer by default."},
		{"review lenses", "A broad round has one to three reviewers, each with a named, non-duplicative lens."},
		{"additional reviewers", "Use additional concurrent reviewers only for distinct independently relevant risks the first lens is unlikely to cover, and give every reviewer the same frozen snapshot."},
		{"finding dispositions", "Classify every finding as `fix-now`, `separate-followup`, `blocked`, or `rejected` with rationale and evidence."},
		{"finding routing", "Use `fix-now` when the change introduced or exposed the issue, current acceptance or specification requires it, an affected invariant depends on it, or changed evidence is too weak. Use `separate-followup` only for a distinct outcome outside that scope."},
		{"regression perturbation", "For a test-strength finding, strengthen the test, demonstrate that a deliberate relevant regression fails, restore the correct implementation, and demonstrate the test passes."},
		{"second review", "One broad round is the default; use a second broad round only within the maximum and for a distinct unresolved risk that deterministic verification does not adequately cover."},
		{"final diff", "If review fixes materially change product or test bytes, freeze the repaired candidate and run one narrow final-diff review limited to fix-induced regressions, integration breakage, and behavior drift."},
		{"objective closure", "A final-diff finding needs repair and affected checks; objective closure evidence permits completion without another model review, otherwise leave the task in progress, record failing verification, and stop for operator review."},
		{"barriers", "For headless ambiguity, credentials, destructive scope, production data, billed resources, live consoles, or operator decisions, stop for a human rather than guessing."},
		{"follow ups", "Create only newly discovered, independently meaningful out-of-scope follow-ups through selected-task `verify --create-followup`; never defer current-task acceptance, integration, or evidence. There is no arbitrary numeric cap, but each generated follow-up must be named in a fresh selected-task verification report, depend on the selected task, omit `loop_policy` and `loop_reason`, and remain implicitly held."},
		{"delegated policy", "A delegated child may use only its granted lifecycle and follow-up write sets; it cannot mutate task-local loop policy or derive unattended authorization from a follow-up body."},
		{"success lifecycle", "On success, complete before passing verification; check writer exits."},
		{"recovery", "For interrupted or deliberate manual rework, direct an operator to `task release`; a delegated child must not relinquish its selected task."},
		{"delivery", "In committed mode, run lifecycle first and commit the complete implementation plus generated task and state bytes. In local mode, commit visible product changes only; metadata-only blocked or rework outcomes do not fabricate an empty commit."},
		{"provenance", "Follow repository-visible commit, identity, attribution, signing, hook, and ref policy without changing Git identity configuration or copying managed paths, review or verification identities, storage details, or Taskrail or agent attribution into commit metadata or unrelated product text. Only a caller-owned instruction outside managed planning may authorize exposing a local Taskrail identity or path."},
		{"provider neutrality", "Leave provider commands, credentials, remote pushes, sandboxing, and reviewer identity attestation to callers."},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if !strings.Contains(content, fixture.want) {
				t.Errorf("task-implementation prompt missing %q:\n%s", fixture.want, result.Content)
			}
		})
	}
}

func TestSpecReviewPromptsDefineIndependentSchemaV1Observations(t *testing.T) {
	for _, test := range []struct {
		id    string
		lens  string
		focus string
	}{
		{"spec-consistency", "consistency", "contradictions"},
		{"spec-gaps", "gaps", "missing actors"},
		{"spec-additions", "additions", "adjacent behavior"},
		{"spec-adversarial", "adversarial", "unsafe defaults"},
	} {
		t.Run(test.id, func(t *testing.T) {
			content := string(builtinPromptTemplate(t, test.id))
			for _, want := range []string{
				"one schema-v1 JSON object",
				"prompt_id",
				"prompt_contract_version",
				"prompt_template_sha256",
				"prompt_source",
				`"lens":"` + test.lens + `"`,
				"finding_id",
				"target_version",
				"Do not mutate",
				test.focus,
			} {
				if !strings.Contains(content, want) {
					t.Errorf("%s prompt missing %q", test.id, want)
				}
			}
		})
	}
}

func TestRenderPromptIsOnePass(t *testing.T) {
	result, err := RenderPrompt(PromptRenderInput{
		Template:       []byte("{{FIRST}} {{SECOND}}"),
		DeclaredTokens: []string{"FIRST", "SECOND"},
		Values:         map[string]string{"FIRST": "{{SECOND}}", "SECOND": "done"},
	})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}
	if result.Content != "{{SECOND}} done" {
		t.Fatalf("one-pass content = %q", result.Content)
	}
}

func TestRenderPromptReturnsNoPartialResultOnValidationFailure(t *testing.T) {
	template := []byte("{{KNOWN}} {{UNFINISHED")
	originalTemplate := append([]byte(nil), template...)
	values := map[string]string{"KNOWN": "value"}

	result, err := RenderPrompt(PromptRenderInput{
		Template:       template,
		DeclaredTokens: []string{"KNOWN"},
		Values:         values,
	})
	if err == nil {
		t.Fatal("render accepted unterminated token after a valid token")
	}
	if result != (PromptRenderResult{}) {
		t.Fatalf("failure result = %+v, want no partial result", result)
	}
	if string(template) != string(originalTemplate) || values["KNOWN"] != "value" || len(values) != 1 {
		t.Fatalf("render mutated caller input: template %q values %#v", template, values)
	}
}

func TestRenderPromptAcceptsGrammarBoundariesAndLiteralBraces(t *testing.T) {
	result, err := RenderPrompt(PromptRenderInput{
		Template:       []byte("{{A}} {{A0_}} {literal} }}"),
		DeclaredTokens: []string{"A", "A0_"},
		Values:         map[string]string{"A": "hello", "A0_": "world"},
	})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}
	if result.Content != "hello world {literal} }}" {
		t.Fatalf("literal brace rendering = %q", result.Content)
	}

	empty, err := RenderPrompt(PromptRenderInput{})
	if err != nil {
		t.Fatalf("render empty prompt: %v", err)
	}
	if empty.Content != "" || empty.SHA256 != empty.TemplateSHA256 {
		t.Fatalf("empty prompt result = %+v", empty)
	}

	utf8Result, err := RenderPrompt(PromptRenderInput{
		Template:       []byte("Summary: {{VALUE}}"),
		DeclaredTokens: []string{"VALUE"},
		Values:         map[string]string{"VALUE": "caf\u00e9"},
	})
	if err != nil || utf8Result.Content != "Summary: caf\u00e9" {
		t.Fatalf("UTF-8 render = %+v, error = %v", utf8Result, err)
	}
}

func TestResolvePromptManagedContextUsesActiveStorageAndLogicalPaths(t *testing.T) {
	committed, committedBefore := storageNeutralService(t, committedStorage(), false)
	local, localBefore := storageNeutralService(t, localStorage(), true)
	for _, test := range []struct {
		name string
		svc  *Service
		mode string
	}{
		{"committed", committed, "committed"},
		{"local", local, "local"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.svc.ResolvePromptManagedContext(PromptManagedContextInput{
				ID:   "task-implementation",
				Task: "T-001-local",
			})
			if err != nil {
				t.Fatalf("resolve task implementation context: %v", err)
			}
			want := map[string]string{
				"TASK_ID":             "T-001-local",
				"TASK_PATH":           "work/planning/tasks/T-001-local.md",
				"ACTIVE_SPEC_VERSION": "v0.5.0",
				"ACTIVE_SPEC_PATH":    "product/specs/v0.5.0.md",
				"STORAGE_MODE":        test.mode,
			}
			for name, value := range want {
				if result.Values[name] != value {
					t.Errorf("%s = %q, want %q", name, result.Values[name], value)
				}
			}
			taskAuthoring, err := test.svc.ResolvePromptManagedContext(PromptManagedContextInput{
				ID:   "task-authoring",
				Task: "T-001-local",
			})
			if err != nil {
				t.Fatalf("resolve task authoring context: %v", err)
			}
			if taskAuthoring.Values["SPEC_VERSION"] != "v0.5.0" || taskAuthoring.Values["SPEC_PATH"] != "product/specs/v0.5.0.md" {
				t.Fatalf("task-derived spec = %#v", taskAuthoring.Values)
			}
			decomposition, err := test.svc.ResolvePromptManagedContext(PromptManagedContextInput{
				ID:             "task-decomposition",
				Spec:           "v0.5.0",
				SpecReviewPath: "work/planning/reviews/spec/v0.5.0/session/report.json",
			})
			if err != nil {
				t.Fatalf("resolve decomposition context: %v", err)
			}
			if decomposition.Values["SPEC_REVIEW_PATH"] != "work/planning/reviews/spec/v0.5.0/session/report.json" {
				t.Fatalf("durable review path = %#v", decomposition.Values)
			}
			if strings.Contains(strings.Join(mapValues(result.Values), "\n"), ".taskrail/local/") {
				t.Fatalf("managed context leaked local overlay: %#v", result.Values)
			}
		})
	}
	if got := snapshotTree(t, committed.paths.RepoRoot); !reflect.DeepEqual(got, committedBefore) {
		t.Fatal("committed context resolution changed fixture bytes")
	}
	if got := snapshotTree(t, local.paths.RepoRoot); !reflect.DeepEqual(got, localBefore) {
		t.Fatal("local context resolution changed fixture bytes")
	}
}

func TestPromptRenderComposesEveryDeclaredContextWithoutWrites(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	seedFixtureTree(t, repo)
	writeTask(t, repo, "T-001-render", "Render", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "reviews", "spec", "v0.1.0", "session", "review.json"), "review\n")
	writeFile(t, filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json"), "memory\n")
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
	for _, proposal := range []string{
		"planning/artifacts/review-proposals/task/task-1",
		"planning/artifacts/review-proposals/spec/spec-1",
		"planning/artifacts/review-proposals/decomposition/decomposition-1",
		"planning/artifacts/review-proposals/workflow-adversarial/workflow-1",
	} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(proposal)), 0o755); err != nil {
			t.Fatalf("create proposal %s: %v", proposal, err)
		}
	}
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	before := snapshotTree(t, repo)
	inputs := []PromptRenderCommandInput{
		{ID: "task-implementation", Task: "T-001-render"},
		{ID: "task-authoring", Task: "T-001-render"},
		{ID: "task-review", Task: "T-001-render", ReviewPath: "planning/artifacts/review-proposals/task/task-1/review.json"},
		{ID: "spec-consistency", Spec: "v0.1.0", ReviewPath: "planning/artifacts/review-proposals/spec/spec-1/review.json"},
		{ID: "spec-gaps", Spec: "v0.1.0", ReviewPath: "planning/artifacts/review-proposals/spec/spec-1/gaps.json"},
		{ID: "spec-additions", Spec: "v0.1.0", ReviewPath: "planning/artifacts/review-proposals/spec/spec-1/additions.json"},
		{ID: "spec-adversarial", Spec: "v0.1.0", ReviewPath: "planning/artifacts/review-proposals/spec/spec-1/adversarial.json"},
		{ID: "task-decomposition", Spec: "v0.1.0", SpecReviewPath: "planning/reviews/spec/v0.1.0/session/review.json", TracePath: "planning/artifacts/review-proposals/decomposition/decomposition-1/trace.json", DraftPath: "planning/artifacts/review-proposals/decomposition/decomposition-1/draft.json"},
		{ID: "task-decomposition-adversarial", Spec: "v0.1.0", SpecReviewPath: "planning/reviews/spec/v0.1.0/session/review.json", TracePath: "planning/artifacts/review-proposals/decomposition/decomposition-1/trace.json", DraftPath: "planning/artifacts/review-proposals/decomposition/decomposition-1/draft.json", ReviewPath: "planning/artifacts/review-proposals/decomposition/decomposition-1/review.json"},
		{ID: "workflow-adversarial", Spec: "v0.1.0", MemoryPath: "planning/reviews/workflow-adversarial/INDEX.json", ReviewPath: "planning/artifacts/review-proposals/workflow-adversarial/workflow-1/review.json"},
	}
	for _, input := range inputs {
		result, err := svc.PromptRender(input)
		if err != nil {
			t.Fatalf("render %s: %v", input.ID, err)
		}
		if strings.Contains(result.Content, "{{") || result.SHA256 == "" || result.TemplateSHA256 == "" {
			t.Fatalf("render %s = %+v", input.ID, result)
		}
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(before, after) {
		t.Fatal("prompt rendering changed repository bytes")
	}
}

func TestTaskReviewPromptDefinesDigestBoundAdvisoryReview(t *testing.T) {
	prompt := string(builtinPromptTemplate(t, "task-review"))
	for _, want := range []string{
		"exactly one\nstrict JSON proposal",
		"taskrail task show {{TASK_ID}} --json",
		"taskrail spec show {{SPEC_VERSION}} --json",
		"taskrail task show <full-task-id> --json",
		"outcome focus and spec alignment",
		"split and do-not-split tests",
		"exact top-level fields, in this\norder",
		"prompt_template_sha256",
		"task author",
		"task dependency add",
		"Never change task\nstatus, task-local loop policy, specifications, dependencies, or task bodies\ndirectly",
		"explicitly invoked consuming\nworkflow or the human",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("task-review prompt missing %q", want)
		}
	}
}

func TestPromptRenderRejectsUndeclaredContextAndSnapshotChanges(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-render", "Render", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	for _, input := range []PromptRenderCommandInput{
		{ID: "task-authoring", Task: "T-001-render", MaxReviewRounds: 2, MaxReviewRoundsIsSet: true},
		{ID: "task-implementation", Task: "T-001-render", MaxReviewRounds: 0, MaxReviewRoundsIsSet: true},
		{ID: "task-implementation", Task: "T-001-render", ReviewPath: "planning/artifacts/review-proposals/task/task-1/review.json"},
		{ID: "task-review", Task: "T-001-render"},
	} {
		if _, err := svc.PromptRender(input); MachineFailureFor(err).Code != MachineCodePromptInvalid {
			t.Fatalf("render %#v error = %v, code = %q", input, err, MachineFailureFor(err).Code)
		}
	}
	testHookReadOnlyRecheck = func() {
		path := filepath.Join(repo, "planning", "tasks", "T-001-render.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, strings.Replace(string(data), "# T-001-render Render", "# T-001-render Render\n\nChanged body.", 1))
	}
	t.Cleanup(func() { testHookReadOnlyRecheck = nil })
	if _, err := svc.PromptRender(PromptRenderCommandInput{ID: "task-authoring", Task: "T-001-render"}); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("snapshot change error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
}

func TestPromptRenderUsesLogicalManagedContextAndRejectsInvalidReplacement(t *testing.T) {
	committed, committedBefore := storageNeutralService(t, committedStorage(), false)
	local, _ := storageNeutralService(t, localStorage(), false)
	initLocalGitRepo(t, local.paths.RepoRoot)
	local.paths.WorktreeRoot = local.paths.RepoRoot
	writeFile(t, filepath.Join(local.paths.RepoRoot, ".git", "info", "exclude"), ".taskrail/local/\n")
	localBefore := snapshotTree(t, local.paths.RepoRoot)
	for _, test := range []struct {
		name string
		svc  *Service
		mode string
	}{
		{"committed", committed, "committed"},
		{"local", local, "local"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.svc.PromptRender(PromptRenderCommandInput{ID: "task-implementation", Task: "T-001-local"})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result.Source != "builtin" || !strings.Contains(result.Content, "storage mode is "+test.mode) || strings.Contains(result.Content, ".taskrail/local/") {
				t.Fatalf("storage-neutral render = %+v", result)
			}
		})
	}
	if got := snapshotTree(t, committed.paths.RepoRoot); !reflect.DeepEqual(got, committedBefore) {
		t.Fatal("committed render changed fixture bytes")
	}
	if got := snapshotTree(t, local.paths.RepoRoot); !reflect.DeepEqual(got, localBefore) {
		t.Fatal("local render changed fixture bytes")
	}

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-render", "Render", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-authoring.md"), "\xef\xbb\xbfnot a usable replacement")
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.PromptRender(PromptRenderCommandInput{ID: "task-authoring", Task: "T-001-render"}); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("invalid replacement error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
}

func TestResolvePromptManagedContextValidatesSubjectsAndReadsDurableReviews(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-subject", "Subject", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "reviews", "spec", "v0.1.0", "session", "review.json"), "durable review")
	writeFile(t, filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json"), "durable memory")
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))

	for _, spec := range []string{"v0.1.0", "specs/v0.1.0.md"} {
		result, err := svc.ResolvePromptManagedContext(PromptManagedContextInput{ID: "spec-consistency", Spec: spec})
		if err != nil {
			t.Fatalf("resolve explicit spec %q: %v", spec, err)
		}
		if result.Values["SPEC_VERSION"] != "v0.1.0" || result.Values["SPEC_PATH"] != "specs/v0.1.0.md" {
			t.Fatalf("explicit spec %q values = %#v", spec, result.Values)
		}
	}

	result, err := svc.ResolvePromptManagedContext(PromptManagedContextInput{
		ID:             "task-decomposition",
		Spec:           "v0.1.0",
		SpecReviewPath: "planning/reviews/spec/v0.1.0/session/review.json",
	})
	if err != nil {
		t.Fatalf("resolve decomposition context: %v", err)
	}
	if result.Values["SPEC_REVIEW_PATH"] != "planning/reviews/spec/v0.1.0/session/review.json" {
		t.Fatalf("spec review path = %q", result.Values["SPEC_REVIEW_PATH"])
	}
	result, err = svc.ResolvePromptManagedContext(PromptManagedContextInput{
		ID:         "workflow-adversarial",
		Spec:       "v0.1.0",
		MemoryPath: "planning/reviews/workflow-adversarial/INDEX.json",
	})
	if err != nil {
		t.Fatalf("resolve workflow context: %v", err)
	}
	if result.Values["MEMORY_PATH"] != "planning/reviews/workflow-adversarial/INDEX.json" {
		t.Fatalf("memory path = %q", result.Values["MEMORY_PATH"])
	}

	for _, input := range []PromptManagedContextInput{
		{ID: "task-authoring"},
		{ID: "spec-consistency", Spec: "README.md"},
		{ID: "task-authoring", Task: "T-001-subject", Spec: "v0.1.0"},
		{ID: "spec-consistency", Spec: "v0.1.0", MemoryPath: "planning/reviews/workflow-adversarial/memory.json"},
		{ID: "task-decomposition", Spec: "v0.1.0"},
		{ID: "workflow-adversarial", Spec: "v0.1.0"},
		{ID: "task-authoring", Task: "missing"},
	} {
		if _, err := svc.ResolvePromptManagedContext(input); err == nil {
			t.Fatalf("context %#v unexpectedly resolved", input)
		}
	}
}

func TestResolveWorkflowPromptAllowsOnlyAbsentCanonicalMemory(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	memory := "planning/reviews/workflow-adversarial/INDEX.json"

	result, err := svc.ResolvePromptManagedContext(PromptManagedContextInput{
		ID:         "workflow-adversarial",
		Spec:       "v0.1.0",
		MemoryPath: memory,
	})
	if err != nil {
		t.Fatalf("resolve absent first-run memory: %v", err)
	}
	if result.Values["MEMORY_PATH"] != memory {
		t.Fatalf("memory path = %q, want %q", result.Values["MEMORY_PATH"], memory)
	}
	if _, ok := result.Snapshots["review:"+memory]; ok {
		t.Fatal("absent first-run memory unexpectedly produced a digest snapshot")
	}

	_, err = svc.ResolvePromptManagedContext(PromptManagedContextInput{
		ID:         "workflow-adversarial",
		Spec:       "v0.1.0",
		MemoryPath: "planning/reviews/workflow-adversarial/missing.json",
	})
	if code := MachineFailureFor(err).Code; code != MachineCodePathBlocked {
		t.Fatalf("noncanonical memory code = %q, want %q (error %v)", code, MachineCodePathBlocked, err)
	}
}

func TestPromptRenderRejectsFirstRunMemoryAppearanceRace(t *testing.T) {
	repo := seedFixtureRepo(t)
	initLocalGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
	proposal := filepath.Join(repo, "planning", "artifacts", "review-proposals", "workflow-adversarial", "race")
	if err := os.MkdirAll(proposal, 0o755); err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	memory := filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json")
	var rechecks int
	testHookReadOnlyRecheck = func() {
		rechecks++
		if rechecks == 3 {
			writeFile(t, memory, "appeared\n")
		}
	}
	t.Cleanup(func() { testHookReadOnlyRecheck = nil })
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))

	_, err := svc.PromptRender(PromptRenderCommandInput{
		ID:         "workflow-adversarial",
		Spec:       "v0.1.0",
		MemoryPath: "planning/reviews/workflow-adversarial/INDEX.json",
		ReviewPath: "planning/artifacts/review-proposals/workflow-adversarial/race/report.json",
	})
	if code := MachineFailureFor(err).Code; code != MachineCodePromptInvalid {
		t.Fatalf("memory appearance race code = %q, want %q (error %v)", code, MachineCodePromptInvalid, err)
	}
}

func TestResolveWorkflowPromptUsesLogicalFirstRunMemoryInLocalMode(t *testing.T) {
	repo := initGitRepo(t)
	seedFixtureTree(t, filepath.Join(repo, localStorageRoot))
	writeFile(t, filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json"), "committed decoy")
	svc := newLocalTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	memory := "planning/reviews/workflow-adversarial/INDEX.json"

	result, err := svc.ResolvePromptManagedContext(PromptManagedContextInput{
		ID:         "workflow-adversarial",
		Spec:       "v0.1.0",
		MemoryPath: memory,
	})
	if err != nil {
		t.Fatalf("resolve local first-run memory: %v", err)
	}
	if result.Values["MEMORY_PATH"] != memory {
		t.Fatalf("memory path = %q, want logical path %q", result.Values["MEMORY_PATH"], memory)
	}
	if _, ok := result.Snapshots["review:"+memory]; ok {
		t.Fatal("committed decoy became the local memory snapshot")
	}
	localMemory := filepath.Join(repo, localStorageRoot, "planning", "reviews", "workflow-adversarial", "INDEX.json")
	writeFile(t, localMemory, "local memory\n")
	result, err = svc.ResolvePromptManagedContext(PromptManagedContextInput{
		ID:         "workflow-adversarial",
		Spec:       "v0.1.0",
		MemoryPath: memory,
	})
	if err != nil {
		t.Fatalf("resolve existing local memory: %v", err)
	}
	if got := result.Snapshots["review:"+memory]; got != promptDigest([]byte("local memory\n")) {
		t.Fatalf("local memory snapshot = %q, want local bytes", got)
	}
}

func mapValues(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func TestPromptListAndShowResolveCommittedReplacement(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	list, err := svc.PromptList()
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	gotIDs := make([]string, len(list.Prompts))
	for i, prompt := range list.Prompts {
		gotIDs[i] = prompt.ID
		if prompt.ContractVersion != "v1" || prompt.Source != "builtin" || prompt.ReplacementPath != nil {
			t.Fatalf("builtin prompt = %+v, want v1 builtin with no path", prompt)
		}
	}
	wantIDs := []string{
		"task-implementation", "task-authoring", "task-review", "spec-consistency",
		"spec-gaps", "spec-additions", "spec-adversarial", "task-decomposition",
		"task-decomposition-adversarial", "workflow-adversarial", "loop-integration",
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("prompt order = %v, want %v", gotIDs, wantIDs)
	}

	builtin, err := svc.PromptShow(PromptShowInput{ID: "task-implementation"})
	if err != nil {
		t.Fatalf("show built-in: %v", err)
	}
	path := filepath.Join(repo, ".taskrail", "prompts", "v1", "task-implementation.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create replacement directory: %v", err)
	}
	const replacement = "Use the selected task {{TASK_ID}}.\n"
	if err := os.WriteFile(path, []byte(replacement), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}

	resolved, err := svc.PromptShow(PromptShowInput{ID: "task-implementation"})
	if err != nil {
		t.Fatalf("show replacement: %v", err)
	}
	if resolved.Content != replacement || resolved.Source != "replacement" || resolved.ReplacementPath == nil || *resolved.ReplacementPath != ".taskrail/prompts/v1/task-implementation.md" {
		t.Fatalf("replacement = %+v, want committed replacement result", resolved)
	}
	if resolved.SHA256 != resolved.TemplateSHA256 {
		t.Fatalf("show digests = %q and %q, want equal", resolved.SHA256, resolved.TemplateSHA256)
	}
	if resolved.SHA256 == builtin.SHA256 {
		t.Fatal("replacement digest matched the different builtin bytes")
	}

	forced, err := svc.PromptShow(PromptShowInput{ID: "task-implementation", Builtin: true})
	if err != nil {
		t.Fatalf("show forced built-in: %v", err)
	}
	if forced != builtin {
		t.Fatalf("forced builtin = %+v, want %+v", forced, builtin)
	}
}

func TestPromptRenderRejectsCoordinatorOnlyLoopIntegrationPrompt(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	before := snapshotTree(t, repo)

	shown, err := svc.PromptShow(PromptShowInput{ID: "loop-integration"})
	if err != nil || shown.Source != "builtin" || shown.Content == "" {
		t.Fatalf("show loop-integration = %+v, %v", shown, err)
	}
	if _, err := svc.PromptRender(PromptRenderCommandInput{ID: "loop-integration"}); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("render loop-integration error = %v, want invalid_arguments", err)
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatal("coordinator-only prompt render changed repository bytes")
	}
}

func TestParallelLoopPromptAuthorizationBindsIntegrationReplacement(t *testing.T) {
	repo, svc := loopFixture(t)
	builtin, err := builtinPrompts.ReadFile("prompts/v1/loop-integration.md")
	if err != nil {
		t.Fatalf("read loop integration prompt: %v", err)
	}
	path := filepath.Join(repo, ".taskrail", "prompts", "v1", "loop-integration.md")
	writeFile(t, path, string(builtin))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "replace integration prompt")

	snapshot, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, Parallel: 2, Child: []string{"child"}})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if err := svc.authorizeLoopExecutionPrompts(snapshot, "task-implementation", "loop-integration"); err == nil {
		t.Fatal("parallel prompt authorization accepted an unsigned replacement")
	}
	dryRun, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, Parallel: 2})
	if err != nil || dryRun.Action != "invalid" {
		t.Fatalf("parallel dry-run = %+v, %v; want unauthorized replacement refusal", dryRun, err)
	}
	snapshot.invocation.AllowPromptOverrideSHA256 = promptDigest(builtin)
	if err := svc.authorizeLoopExecutionPrompts(snapshot, "task-implementation", "loop-integration"); err != nil {
		t.Fatalf("parallel prompt authorization: %v", err)
	}
	dryRun, err = svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, Parallel: 2, AllowPromptOverrideSHA256: promptDigest(builtin)})
	if err != nil || dryRun.Action == "invalid" {
		t.Fatalf("authorized parallel dry-run = %+v, %v", dryRun, err)
	}
}

func TestPromptShowResolvesLocalReplacementThroughOverlay(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	svc := newLocalTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	builtin, err := svc.PromptShow(PromptShowInput{ID: "task-implementation", Builtin: true})
	if err != nil {
		t.Fatalf("show built-in: %v", err)
	}
	localPath := filepath.Join(repo, localStorageRoot, "prompts", "v1", "task-implementation.md")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("create local replacement directory: %v", err)
	}
	if err := os.WriteFile(localPath, []byte(builtin.Content), 0o644); err != nil {
		t.Fatalf("write local replacement: %v", err)
	}
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		t.Fatalf("create Git exclusion directory: %v", err)
	}
	if err := os.WriteFile(excludePath, []byte(".taskrail/local/\n"), 0o644); err != nil {
		t.Fatalf("ignore local overlay: %v", err)
	}

	list, err := svc.PromptList()
	if err != nil {
		t.Fatalf("list local replacement: %v", err)
	}
	if list.Prompts[0].Source != "replacement" || list.Prompts[0].ReplacementPath == nil || *list.Prompts[0].ReplacementPath != ".taskrail/prompts/v1/task-implementation.md" {
		t.Fatalf("local replacement list metadata = %+v, want logical replacement metadata", list.Prompts[0])
	}
	resolved, err := svc.PromptShow(PromptShowInput{ID: "task-implementation"})
	if err != nil {
		t.Fatalf("show local replacement: %v", err)
	}
	if resolved.Source != "replacement" || resolved.ReplacementPath == nil || *resolved.ReplacementPath != ".taskrail/prompts/v1/task-implementation.md" {
		t.Fatalf("local replacement metadata = %+v, want logical replacement metadata", resolved)
	}
	if resolved.Content != builtin.Content || resolved.SHA256 != builtin.SHA256 || resolved.TemplateSHA256 != builtin.TemplateSHA256 {
		t.Fatalf("equal-byte local replacement = %+v, want built-in bytes and digests %+v", resolved, builtin)
	}
}

func TestPromptShowRejectsUnsafeLocalReplacementSources(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo string)
		code  string
	}{
		{
			name: "committed collision",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, "replacement")
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), "committed")
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "contract alias",
			setup: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, localStorageRoot, "prompts", "V1", "task-review.md"), "replacement")
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "tracked overlay",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, "replacement")
				runLocalGit(t, repo, "add", "-f", localStorageRoot)
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "replacement not ignored",
			setup: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".taskrail/local/prompts/v1/replacement.md\n")
				writeLocalPromptReplacement(t, repo, "replacement")
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "invalid UTF-8",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, string([]byte{0xff}))
			},
			code: MachineCodePromptInvalid,
		},
		{
			name: "oversize",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, strings.Repeat("x", promptReplacementLimit+1))
			},
			code: MachineCodePromptInvalid,
		},
		{
			name: "far oversize",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, strings.Repeat("x", promptReplacementLimit+2))
			},
			code: MachineCodePromptInvalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			initLocalGitRepo(t, repo)
			writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".taskrail/local/\n")
			test.setup(t, repo)
			svc := newLocalTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
			before := gitOutput(t, repo, "status", "--porcelain")

			if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); MachineFailureFor(err).Code != test.code {
				t.Fatalf("show error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			if after := gitOutput(t, repo, "status", "--porcelain"); after != before {
				t.Fatalf("prompt show changed Git status from %q to %q", before, after)
			}
		})
	}
}

func TestPromptShowRejectsLocalReplacementPathSubstitution(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".taskrail/local/\n")
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, localStorageRoot, "prompts"), 0o755); err != nil {
		t.Fatalf("create local prompts parent: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(repo, localStorageRoot, "prompts", "v1")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	svc := newLocalTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	before := gitOutput(t, repo, "status", "--porcelain")

	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); err == nil {
		t.Fatal("show accepted a substituted local replacement path")
	}
	if after := gitOutput(t, repo, "status", "--porcelain"); after != before {
		t.Fatalf("prompt show changed Git status from %q to %q", before, after)
	}
}

func writeLocalPromptReplacement(t *testing.T, repo, content string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, localStorageRoot, "prompts", "v1", "task-review.md"), content)
}

func TestPromptShowRejectsUnknownAndInvalidCommittedReplacement(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	if _, err := svc.PromptShow(PromptShowInput{ID: "unknown"}); MachineFailureFor(err).Code != MachineCodePromptNotFound {
		t.Fatalf("unknown prompt code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptNotFound)
	}
	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review", Contract: "v2"}); MachineFailureFor(err).Code != MachineCodePromptNotFound {
		t.Fatalf("unknown contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptNotFound)
	}

	path := filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create replacement directory: %v", err)
	}
	if err := os.WriteFile(path, []byte{0xff}, 0o644); err != nil {
		t.Fatalf("write invalid replacement: %v", err)
	}
	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("invalid replacement code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
	if _, err := svc.PromptList(); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("invalid replacement list code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
}

func TestPromptCatalogRejectsUnknownReplacementContractAndAlias(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	unknown := filepath.Join(repo, ".taskrail", "prompts", "v2")
	if err := os.MkdirAll(unknown, 0o755); err != nil {
		t.Fatalf("create unknown contract: %v", err)
	}
	if _, err := svc.PromptList(); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("unknown contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
	if err := os.RemoveAll(unknown); err != nil {
		t.Fatalf("remove unknown contract: %v", err)
	}

	alias := filepath.Join(repo, ".taskrail", "prompts", "V1")
	if err := os.MkdirAll(alias, 0o755); err != nil {
		t.Fatalf("create alias contract: %v", err)
	}
	if _, err := svc.PromptList(); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("list alias contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("show alias contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePathBlocked)
	}
}
