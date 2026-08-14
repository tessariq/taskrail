package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSharedReadersAreStorageNeutral(t *testing.T) {
	committed, committedBefore := storageNeutralService(t, committedStorage(), false)
	local, localBefore := storageNeutralService(t, localStorage(), true)

	committedModel := readStorageNeutralModel(t, committed)
	localModel := readStorageNeutralModel(t, local)
	if !reflect.DeepEqual(committedModel, localModel) {
		t.Fatalf("committed/local semantic models differ:\ncommitted: %#v\nlocal: %#v", committedModel, localModel)
	}
	if got := snapshotTree(t, committed.paths.RepoRoot); !reflect.DeepEqual(got, committedBefore) {
		t.Fatal("committed read-only reports changed fixture bytes")
	}
	if got := snapshotTree(t, local.paths.RepoRoot); !reflect.DeepEqual(got, localBefore) {
		t.Fatal("local read-only reports changed fixture bytes")
	}
}

type storageNeutralModel struct {
	Validation ValidationResult
	SpecList   SpecListResult
	SpecShow   SpecShowResult
	Coverage   CoverageReport
	Gaps       GapReport
	Status     StatusReport
	Stats      StatsReport
	Graph      string
	TaskPath   string
	StateBody  string
	Review     string
}

func readStorageNeutralModel(t *testing.T, svc *Service) storageNeutralModel {
	t.Helper()
	validation, err := svc.Validate()
	if err != nil || !validation.Valid {
		t.Fatalf("validate: result=%+v err=%v", validation, err)
	}
	list, err := svc.SpecList()
	if err != nil {
		t.Fatalf("spec list: %v", err)
	}
	show, err := svc.SpecShow("v0.5.0", false)
	if err != nil {
		t.Fatalf("spec show: %v", err)
	}
	coverage, err := svc.Coverage()
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	gaps, err := svc.CoverageGaps()
	if err != nil {
		t.Fatalf("coverage gaps: %v", err)
	}
	status, err := svc.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	// Storage intentionally reports the physical context; all semantic fields
	// around it must remain byte-equivalent.
	status.Storage = StatusStorage{}
	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	graph, err := svc.DependencyGraph("dot")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load model: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	reviewPath := "work/planning/reviews/spec/v0.5.0/session/report.json"
	review, err := readPublishedReview(WorkflowEvidenceContext{
		RepoRoot:     svc.paths.RepoRoot,
		Storage:      svc.paths.Storage,
		PlanningDir:  svc.paths.LogicalPlanningDir,
		ArtifactsDir: filepath.ToSlash(relPath(svc.paths.RepoRoot, svc.paths.ArtifactsDir)),
	}, reviewPath)
	if err != nil {
		t.Fatalf("read published review: %v", err)
	}
	return storageNeutralModel{
		Validation: validation,
		SpecList:   list,
		SpecShow:   show,
		Coverage:   coverage,
		Gaps:       gaps,
		Status:     status,
		Stats:      stats,
		Graph:      graph,
		TaskPath:   tasks[0].Path,
		StateBody:  renderStateBody(state.Frontmatter, tasks),
		Review:     string(review),
	}
}

func storageNeutralService(t *testing.T, storage StorageContext, decoys bool) (*Service, map[string]string) {
	t.Helper()
	repo := initGitRepo(t)
	cfg := LayoutConfig{LayoutVersion: layout2Version, SpecsDir: "product/specs", PlanningDir: "work/planning"}
	paths := pathsFromLayout(repo, cfg, storage)
	writeFile(t, filepath.Join(paths.SpecsDir, "README.md"), "# Specs\n")
	writeFile(t, filepath.Join(paths.SpecsDir, "v0.5.0.md"), "# v0.5.0\n\n## Potential Features\n\n### Local Planning\n\nRequirements:\n\n- Storage neutral.\n")
	writeFile(t, paths.StateFile, `---
schema_version: 1
updated_at: "2026-08-14T00:00:00Z"
active_spec_version: v0.5.0
active_spec_path: product/specs/v0.5.0.md
current_task: ""
current_task_title: ""
status_summary: idle
blockers: []
next_action: Select the next eligible task
last_verification_result: Not yet run
relevant_artifacts: []
continuation_notes: []
---

# STATE
`)
	writeFile(t, filepath.Join(paths.TasksDir, "T-001-local.md"), `---
id: T-001-local
title: Local task
status: todo
priority: high
spec_ref: product/specs/v0.5.0.md#local-planning
dependencies: []
updated_at: "2026-08-14T00:00:00Z"
---

# T-001-local Local task

## Description

Exercise storage-neutral readers.
`)
	writeFile(t, filepath.Join(paths.PlanningDir, "reviews", "spec", "v0.5.0", "session", "report.json"), "review bytes\n")
	if decoys {
		writeFile(t, filepath.Join(repo, "product", "specs", "v0.5.0.md"), "# decoy without requested anchor\n")
		writeFile(t, filepath.Join(repo, "work", "planning", "STATE.md"), "decoy state\n")
		writeFile(t, filepath.Join(repo, "work", "planning", "tasks", "T-001-renamed.md"), "decoy task\n")
		writeFile(t, filepath.Join(repo, "work", "planning", "reviews", "spec", "v0.5.0", "session", "report.json"), "decoy review\n")
	}
	svc := &Service{paths: paths, now: func() time.Time { return time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC) }}
	return svc, snapshotTree(t, repo)
}

func TestStorageNeutralArtifactAndRenameGuardsUseLogicalPaths(t *testing.T) {
	svc, _ := storageNeutralService(t, localStorage(), true)
	taskPath := filepath.Join(svc.paths.TasksDir, "T-001-local.md")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, taskPath, strings.Replace(string(data), "Exercise storage-neutral readers.",
		"See work/planning/artifacts/verify/T-001-local/run/report.json for evidence.", 1))

	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !hasViolation(validation.Violations, "task T-001-local body references gitignored artifact path work/planning/artifacts/verify/T-001-local/run/report.json") {
		t.Fatalf("custom logical artifact violation missing: %v", validation.Violations)
	}
	before := snapshotTree(t, svc.paths.RepoRoot)
	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-001-local", Slug: "renamed", SlugExplicit: true, DryRun: true})
	if err != nil {
		t.Fatalf("rename preview: %v", err)
	}
	if result.Changes[1].From != "work/planning/tasks/T-001-local.md" || result.Changes[1].To != "work/planning/tasks/T-001-renamed.md" {
		t.Fatalf("rename paths leaked physical storage: %+v", result.Changes[1])
	}
	if got := snapshotTree(t, svc.paths.RepoRoot); !reflect.DeepEqual(got, before) {
		t.Fatal("rename preview changed fixture bytes")
	}
}

func TestSharedReadersRefuseUnsupportedStorageCapability(t *testing.T) {
	svc, _ := storageNeutralService(t, StorageContext{Mode: "remote", Root: "remote"}, false)
	_, err := svc.Status()
	if err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported || !strings.Contains(err.Error(), "storage context") {
		t.Fatalf("unsupported storage error = %v", err)
	}
}
