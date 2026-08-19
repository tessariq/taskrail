package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Init's published result is fixed by specs/contracts/v0.5.0-machine-api.md.
// These tests compare whole documents rather than single members, so a dropped,
// renamed, or reordered field fails here instead of reaching an agent.

// initJSON marshals the result the way the envelope publishes it.
func initJSON(t *testing.T, result InitResult) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal init result: %v", err)
	}
	return string(encoded)
}

// markerDigestOnDisk digests the marker bytes init actually wrote, so a reported
// candidate digest is checked against the file rather than against the same
// computation that produced it.
func markerDigestOnDisk(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(markerFile(repo))
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestInitFreshResultMatchesTheContract(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	want := `{"outcome":"created","from_version":0,"to_version":1,"applied":true,` +
		`"storage_mode":"committed",` +
		`"config":{"path":".taskrail/config.yml","action":"create","candidate_sha256":"` + markerDigestOnDisk(t, repo) + `"},` +
		`"writes":[` +
		`{"path":".taskrail/config.yml","kind":"config","action":"create"},` +
		`{"path":"planning/NOTES.md","kind":"note","action":"create"},` +
		`{"path":"planning/STATE.md","kind":"state","action":"create"},` +
		`{"path":"specs/README.md","kind":"spec","action":"create"},` +
		`{"path":"specs/v0.1.0.md","kind":"spec","action":"create"}],` +
		`"notes":[{"path":"planning/NOTES.md","file_action":"create_template",` +
		`"continuation_action":null,"continuation_choices":[]}],` +
		`"skills":[],"skill_exclusions":[],"continuation_notes":[],"validation":null}`
	if got := initJSON(t, result); got != want {
		t.Fatalf("init result =\n%s\nwant\n%s", got, want)
	}
}

// Adoption writes the marker and nothing else, so the layout it finds is
// reported as preserved and the absent sidecar as untouched rather than as a
// write this outcome would make.
func TestInitAdoptedReportsPreservedLayout(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	want := []WriteEntry{
		{Path: ".taskrail/config.yml", Kind: writeKindConfig, Action: writeActionCreate},
		{Path: "planning/STATE.md", Kind: writeKindState, Action: writeActionPreserve},
		{Path: "specs/README.md", Kind: writeKindSpec, Action: writeActionPreserve},
		{Path: "specs/v0.1.0.md", Kind: writeKindSpec, Action: writeActionPreserve},
	}
	if !slices.Equal(result.Writes, want) {
		t.Fatalf("writes = %+v, want %+v", result.Writes, want)
	}
	if action := result.Notes[0].FileAction; action != noteActionNone {
		t.Fatalf("notes file_action = %q, want %q", action, noteActionNone)
	}
	if result.FromVersion != 0 {
		t.Fatalf("from_version = %d, want 0 for an absent prior marker", result.FromVersion)
	}
}

// A preview must promise exactly what the apply after it delivers: same
// candidate paths, same note choices, same marker candidate.
func TestInitMigrationPreviewAndApplyExposeTheSameCandidates(t *testing.T) {
	t.Parallel()

	marker := "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n"
	previewRepo := seedFixtureRepo(t)
	writeFile(t, markerFile(previewRepo), marker)
	applyRepo := seedFixtureRepo(t)
	writeFile(t, markerFile(applyRepo), marker)

	at := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	preview, err := newTestService(t, previewRepo, at).Init(InitInput{})
	if err != nil {
		t.Fatalf("init preview: %v", err)
	}
	applied, err := newTestService(t, applyRepo, at).Init(InitInput{Apply: true})
	if err != nil {
		t.Fatalf("init apply: %v", err)
	}

	if preview.Outcome != InitMigrationPreview || preview.Applied {
		t.Fatalf("preview outcome = %q applied=%v", preview.Outcome, preview.Applied)
	}
	if applied.Outcome != InitMigrated || !applied.Applied {
		t.Fatalf("apply outcome = %q applied=%v", applied.Outcome, applied.Applied)
	}
	if !slices.Equal(preview.Writes, applied.Writes) {
		t.Fatalf("preview writes %+v differ from applied %+v", preview.Writes, applied.Writes)
	}
	if !slices.Equal(preview.Notes[0].ContinuationChoices, applied.Notes[0].ContinuationChoices) {
		t.Fatalf("preview choices %v differ from applied %v",
			preview.Notes[0].ContinuationChoices, applied.Notes[0].ContinuationChoices)
	}
	if preview.Config != applied.Config {
		t.Fatalf("preview config %+v differs from applied %+v", preview.Config, applied.Config)
	}
	if preview.Config.Action != configActionMigrate {
		t.Fatalf("config action = %q, want %q", preview.Config.Action, configActionMigrate)
	}
	// The candidate digest is a promise about bytes, so it must match the marker
	// the apply actually wrote.
	if applied.Config.CandidateSHA256 != markerDigestOnDisk(t, applyRepo) {
		t.Fatalf("candidate digest %q does not match the written marker", applied.Config.CandidateSHA256)
	}
	// Only the applied outcome re-runs validation.
	if preview.Validation != nil {
		t.Fatalf("preview reported validation %+v", preview.Validation)
	}
	if applied.Validation == nil || !applied.Validation.Valid {
		t.Fatalf("apply validation = %+v, want valid", applied.Validation)
	}
}

// The continuation-note choices report what an operator could actually select:
// extraction needs notes to move and an absent destination, because an existing
// human-owned sidecar is never appended to.
func TestInitReportsContinuationNoteChoices(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		seedSidecar bool
		want        []string
	}{
		{name: "absent sidecar offers both", want: []string{"extract", "drop"}},
		{name: "existing sidecar offers drop only", seedSidecar: true, want: []string{"drop"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := seedFixtureRepo(t)
			writeFile(t, markerFile(repo), "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n")
			if tc.seedSidecar {
				writeFile(t, notesFile(repo), "# Repository Notes\n")
			}

			svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
			result, err := svc.Init(InitInput{})
			if err != nil {
				t.Fatalf("init: %v", err)
			}
			note := result.Notes[0]
			if !slices.Equal(note.ContinuationChoices, tc.want) {
				t.Fatalf("continuation_choices = %v, want %v", note.ContinuationChoices, tc.want)
			}
			if note.ContinuationAction != nil {
				t.Fatalf("continuation_action = %q, want null before an operator selects one", *note.ContinuationAction)
			}
			// The fixture state's notes are reported verbatim, in decoded order.
			if !slices.Equal(result.ContinuationNotes, []string{"Fixture repo."}) {
				t.Fatalf("continuation_notes = %v", result.ContinuationNotes)
			}
		})
	}
}

// A repository with no continuation notes offers no choice at all, rather than
// an inapplicable one.
func TestInitReportsNoChoicesWithoutContinuationNotes(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if len(result.Notes[0].ContinuationChoices) != 0 || len(result.ContinuationNotes) != 0 {
		t.Fatalf("notes = %+v, continuation_notes = %v", result.Notes, result.ContinuationNotes)
	}
}

func TestInitSkillInventoryIsEmptyWithoutTheOptIn(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if len(result.Skills) != 0 || len(result.SkillExclusions) != 0 {
		t.Fatalf("skills = %+v, exclusions = %+v, want both empty", result.Skills, result.SkillExclusions)
	}
	for _, dir := range shippableSkillTargets {
		if dirExists(filepath.Join(repo, dir)) {
			t.Fatalf("default init created the assistant directory %s", dir)
		}
	}
}

// A committed installation reports every packaged file at its normal discovery
// path and owns no exclusion; re-running preserves, and a forced refresh over a
// diverged file reports it as refreshed.
func TestInitCommittedSkillInventory(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatalf("packaged skill files: %v", err)
	}
	wantCount := len(files) * len(shippableSkillTargets)

	installed, err := svc.Init(InitInput{WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("init --with-skills: %v", err)
	}
	assertSkillInventory(t, installed.Skills, wantCount, writeActionCreate)
	if len(installed.SkillExclusions) != 0 {
		t.Fatalf("committed install reported exclusions %+v", installed.SkillExclusions)
	}

	preserved, err := svc.Init(InitInput{WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("re-run init --with-skills: %v", err)
	}
	assertSkillInventory(t, preserved.Skills, wantCount, writeActionPreserve)

	// One diverged file, refreshed under --force, is the only entry that changes.
	diverged := filepath.Join(repo, filepath.FromSlash(installed.Skills[0].Path))
	writeFile(t, diverged, "---\nname: edited\ndescription: edited\n---\n")
	refreshed, err := svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("forced refresh: %v", err)
	}
	if refreshed.Skills[0].Action != writeActionRefresh {
		t.Fatalf("skills[0] = %+v, want a refresh", refreshed.Skills[0])
	}
	for _, skill := range refreshed.Skills[1:] {
		if skill.Action != writeActionPreserve {
			t.Fatalf("forced refresh rewrote unmodified %+v", skill)
		}
	}
}

// A local installation keeps the normal discovery paths and adds one exact
// exclusion per packaged-skill subtree in each supported assistant root.
func TestInitLocalSkillInventoryReportsExclusions(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newLocalTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("init --with-skills: %v", err)
	}
	if result.StorageMode != "local" {
		t.Fatalf("storage_mode = %q, want local", result.StorageMode)
	}
	names, err := packagedSkillNames()
	if err != nil {
		t.Fatalf("packaged skill names: %v", err)
	}
	if len(result.SkillExclusions) != len(names)*len(shippableSkillTargets) {
		t.Fatalf("exclusions = %+v, want one subtree per packaged skill per assistant root", result.SkillExclusions)
	}
	for i, exclusion := range result.SkillExclusions {
		if strings.Contains(exclusion.Path, localStorageRoot) {
			t.Fatalf("exclusion %+v carries the local overlay prefix", exclusion)
		}
		if i > 0 && result.SkillExclusions[i-1].Path >= exclusion.Path {
			t.Fatalf("exclusions are not in path order at %q", exclusion.Path)
		}
	}
	for _, skill := range result.Skills {
		if strings.Contains(skill.Path, localStorageRoot) {
			t.Fatalf("skill %+v was materialized beneath the local overlay", skill)
		}
	}
}

func assertSkillInventory(t *testing.T, skills []InitSkill, wantCount int, wantAction string) {
	t.Helper()
	if len(skills) != wantCount {
		t.Fatalf("skills = %d entries, want %d", len(skills), wantCount)
	}
	for i, skill := range skills {
		if skill.Action != wantAction {
			t.Fatalf("skill %+v action = %q, want %q", skill, skill.Action, wantAction)
		}
		if i > 0 && skills[i-1].Path >= skill.Path {
			t.Fatalf("skills are not in path order at %q", skill.Path)
		}
	}
}

// A destination init cannot write is a refusal, not an outcome carrying a
// "refused" action: the caller gets the registered error code instead of a
// result it would have to inspect for failure.
func TestInitRefusesBlockedNotesDestination(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "planning", notesFileName), 0o755); err != nil {
		t.Fatalf("seed blocked destination: %v", err)
	}
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	_, err := svc.Init(InitInput{})
	if err == nil {
		t.Fatal("init must refuse a notes destination it cannot write")
	}
	if code := MachineFailureFor(err).Code; code != MachineCodePathBlocked {
		t.Fatalf("error code = %q, want %q", code, MachineCodePathBlocked)
	}
}

// A blocked skill destination is discovered by the shared transaction before
// publication. No earlier skill, backup, marker, or scaffold may leak out.
func TestInitBlockedSkillInstallPublishesNothing(t *testing.T) {
	repo := initGitRepo(t)
	names, err := packagedSkillNames()
	if err != nil {
		t.Fatalf("packaged skill names: %v", err)
	}
	if len(names) < 2 {
		t.Skipf("this scenario needs at least two packaged skills, have %d", len(names))
	}
	first, second := names[0], names[1]

	// Diverging one skill would normally create a backup and replacement. A later
	// blocked subtree must prevent both from publishing.
	diverged := filepath.Join(repo, shippableSkillTargets[0], first, skillFileName)
	writeFile(t, diverged, "---\nname: edited\ndescription: edited\n---\n")
	blocked := filepath.Join(repo, shippableSkillTargets[1], second)
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("seed read-only skill subtree: %v", err)
	}
	// Probe that the platform actually enforces the injection rather than
	// assuming it: root and native Windows ignore a directory's read-only bit,
	// and there the scenario cannot fire at all.
	requireReadOnlyDirBlocksWrites(t, blocked)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	before, readErr := os.ReadFile(diverged)
	if readErr != nil {
		t.Fatalf("read diverged skill: %v", readErr)
	}
	_, err = svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"})
	if err == nil {
		t.Fatal("a blocked skill subtree must fail the installation")
	}
	after, readErr := os.ReadFile(diverged)
	if readErr != nil || !slices.Equal(before, after) {
		t.Fatalf("diverged skill changed: err=%v", readErr)
	}
	if backups, globErr := filepath.Glob(diverged + ".bak.*"); globErr != nil || len(backups) != 0 {
		t.Fatalf("failed transaction left backups %v, err=%v", backups, globErr)
	}
	if _, statErr := os.Stat(markerFile(repo)); !os.IsNotExist(statErr) {
		t.Fatalf("failed transaction published marker: %v", statErr)
	}
}
