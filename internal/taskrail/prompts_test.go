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
	writeFile(t, filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "memory.json"), "memory\n")
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
		{ID: "workflow-adversarial", Spec: "v0.1.0", MemoryPath: "planning/reviews/workflow-adversarial/memory.json", ReviewPath: "planning/artifacts/review-proposals/workflow-adversarial/workflow-1/review.json"},
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
	writeFile(t, filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "memory.json"), "durable memory")
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
		MemoryPath: "planning/reviews/workflow-adversarial/memory.json",
	})
	if err != nil {
		t.Fatalf("resolve workflow context: %v", err)
	}
	if result.Values["MEMORY_PATH"] != "planning/reviews/workflow-adversarial/memory.json" {
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
		"task-decomposition-adversarial", "workflow-adversarial",
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
