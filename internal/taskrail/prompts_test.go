package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

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
