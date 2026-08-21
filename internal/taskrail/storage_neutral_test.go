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

func TestStructuralWritersUseLocalStorageAndLogicalPaths(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("initialize local storage: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	svc.now = func() time.Time { return time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC) }

	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), "committed state decoy\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), "committed spec decoy\n")
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-001-decoy.md"), "committed task decoy\n")
	decoyState := readFileString(t, filepath.Join(repo, "planning", "STATE.md"))
	decoySpec := readFileString(t, filepath.Join(repo, "specs", "v0.1.0.md"))
	decoyTask := readFileString(t, filepath.Join(repo, "planning", "tasks", "T-001-decoy.md"))

	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load local state: %v", err)
	}
	state.Body = "# STATE\n"
	if err := svc.saveState(state); err != nil {
		t.Fatalf("seed stale local state: %v", err)
	}
	beforePreview := snapshotTree(t, repo)
	dry, err := svc.Repair(RepairInput{})
	if err != nil {
		t.Fatalf("preview local repair: %v", err)
	}
	if dry.Applied || len(dry.BodyDiff) == 0 {
		t.Fatalf("local repair preview = %+v, want unapplied state correction", dry)
	}
	if got := snapshotTree(t, repo); !reflect.DeepEqual(got, beforePreview) {
		t.Fatal("local repair preview changed fixture bytes")
	}

	if _, err := svc.Repair(RepairInput{Apply: true}); err != nil {
		t.Fatalf("repair local state: %v", err)
	}
	added, err := svc.AddSpec("v0.6.0")
	if err != nil {
		t.Fatalf("add local spec: %v", err)
	}
	if added.SpecPath != "specs/v0.6.0.md" || added.ReadmePath != "specs/README.md" {
		t.Fatalf("add result exposes physical local paths: %+v", added)
	}
	addedBytes := readFileString(t, filepath.Join(svc.paths.SpecsDir, "v0.6.0.md"))
	if _, err := svc.AddSpec("v0.6.0"); err == nil {
		t.Fatal("local add must refuse an existing local spec")
	}
	if got := readFileString(t, filepath.Join(svc.paths.SpecsDir, "v0.6.0.md")); got != addedBytes {
		t.Fatal("rejected local add changed its existing spec")
	}

	activated, err := svc.ActivateSpec("v0.6.0")
	if err != nil {
		t.Fatalf("activate local spec: %v", err)
	}
	if activated.ActiveSpecPath != "specs/v0.6.0.md" {
		t.Fatalf("activation persisted physical local path: %+v", activated)
	}

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "planning",
		Source:        "imported.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "Local import."}},
		Tasks: []TaskDraft{
			{Key: "alpha", Title: "Alpha", SpecRef: "specs/imported.md#overview"},
			{Key: "beta", Title: "Beta", SpecRef: "specs/imported.md#overview", Dependencies: []string{"alpha"}},
		},
	}
	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: writeDraftFile(t, svc.paths.RepoRoot, "draft.json", draft)})
	if err != nil {
		t.Fatalf("apply local import: %v", err)
	}
	if result.SpecPath != "specs/imported.md" {
		t.Fatalf("import spec path = %q, want logical path", result.SpecPath)
	}
	for _, task := range result.Tasks {
		if !strings.HasPrefix(task.Path, "planning/tasks/") || strings.Contains(task.Path, localStorageRoot) {
			t.Fatalf("import task result exposes physical local path: %+v", task)
		}
		physical, err := svc.paths.physicalManagedPath(task.Path)
		if err != nil {
			t.Fatalf("resolve local task path %q: %v", task.Path, err)
		}
		if _, err := os.Stat(physical); err != nil {
			t.Fatalf("imported task missing from local storage: %v", err)
		}
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(task.Path))); !os.IsNotExist(err) {
			t.Fatalf("imported task leaked into committed storage: %v", err)
		}
	}

	localState := readFileString(t, svc.paths.StateFile)
	if strings.Contains(localState, localStorageRoot) || !strings.Contains(localState, "`specs/v0.6.0.md`") {
		t.Fatalf("local state does not retain logical active spec path:\n%s", localState)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.SpecsDir, "v0.6.0.md")); err != nil {
		t.Fatalf("added spec was not written below local storage: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.SpecsDir, "imported.md")); err != nil {
		t.Fatalf("imported spec was not written below local storage: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.SpecsDir, "README.md")); err != nil {
		t.Fatalf("spec index was not written below local storage: %v", err)
	}
	for _, logical := range []string{"specs/README.md", "specs/v0.6.0.md", "specs/imported.md"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(logical))); !os.IsNotExist(err) {
			t.Fatalf("spec leaked into committed storage at %s: %v", logical, err)
		}
	}
	if got := readFileString(t, filepath.Join(repo, "planning", "STATE.md")); got != decoyState {
		t.Fatal("repair or activation changed committed state decoy")
	}
	if got := readFileString(t, filepath.Join(repo, "specs", "v0.1.0.md")); got != decoySpec {
		t.Fatal("writer changed committed spec decoy")
	}
	if got := readFileString(t, filepath.Join(repo, "planning", "tasks", "T-001-decoy.md")); got != decoyTask {
		t.Fatal("writer changed committed task decoy")
	}
}

func TestAddSpecLocalReadErrorUsesLogicalPath(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("initialize local storage: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	readmePath := filepath.Join(svc.paths.SpecsDir, "README.md")
	if err := os.Remove(readmePath); err != nil {
		t.Fatalf("remove local spec index: %v", err)
	}
	if err := os.Mkdir(readmePath, 0o755); err != nil {
		t.Fatalf("block local spec index read: %v", err)
	}

	_, err = svc.AddSpec("v0.6.0")
	if err == nil {
		t.Fatal("expected unreadable local spec index to refuse add")
	}
	if !strings.Contains(err.Error(), "specs/README.md") || strings.Contains(err.Error(), localStorageRoot) {
		t.Fatalf("local add error exposed physical path: %v", err)
	}
}

func TestLocalImportPreservesRacedSpecDestination(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("initialize local storage: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "spec",
		Source:        "raced.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "candidate"}},
	}
	path := writeDraftFile(t, repo, "draft.json", draft)
	destination := filepath.Join(svc.paths.SpecsDir, "raced.md")
	testHookImportCandidateBuilt = func() {
		writeFile(t, destination, "# authored local spec\n")
	}
	t.Cleanup(func() { testHookImportCandidateBuilt = nil })

	_, err = svc.ApplyImportDraft(ApplyDraftInput{DraftPath: path})
	if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
		t.Fatalf("local import with destination race = %v, want write_conflict", err)
	}
	if got := readFileString(t, destination); got != "# authored local spec\n" {
		t.Fatalf("local import overwrote raced destination: %q", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "specs", "raced.md")); !os.IsNotExist(err) {
		t.Fatalf("local import created committed raced destination: %v", err)
	}
}

func TestSharedReadersRefuseUnsupportedStorageCapability(t *testing.T) {
	svc, _ := storageNeutralService(t, StorageContext{Mode: "remote", Root: "remote"}, false)
	_, err := svc.Status()
	if err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported || !strings.Contains(err.Error(), "storage context") {
		t.Fatalf("unsupported storage error = %v", err)
	}
}
