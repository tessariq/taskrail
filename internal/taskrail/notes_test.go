package taskrail

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func planningDirOf(repo string) string {
	return filepath.Join(repo, "planning")
}

func notesFile(repo string) string {
	return filepath.Join(planningDirOf(repo), notesFileName)
}

func TestInitFreshCreatesNotesTemplate(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if got := readFileString(t, notesFile(repo)); got != starterNotes() {
		t.Fatalf("notes template = %q, want %q", got, starterNotes())
	}
	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("sidecar made the repository invalid: %v", validation.Violations)
	}
}

// The sidecar is human-owned, so a second init must leave whatever the human put
// there byte-for-byte, not restore the template over it.
func TestInitPreservesExistingNotesByteForByte(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	human := "# Repository Notes\n\nHand-authored context the operator owns.\n"
	writeFile(t, notesFile(repo), human)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if got := readFileString(t, notesFile(repo)); got != human {
		t.Fatalf("init rewrote human notes: got %q, want %q", got, human)
	}
}

// Preview is write-free, so both dry-run paths must report the sidecar as a
// candidate change and leave the destination absent.
func TestInitPreviewReportsNotesCandidateWithoutWriting(t *testing.T) {
	t.Parallel()

	want := "create planning/" + notesFileName

	t.Run("migration", func(t *testing.T) {
		t.Parallel()
		repo := seedFixtureRepo(t)
		writeFile(t, markerFile(repo), "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n")

		svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
		result, err := svc.Init(false)
		if err != nil {
			t.Fatalf("init dry run: %v", err)
		}
		if !slices.Contains(result.Changes, want) {
			t.Fatalf("changes = %v, want one entry %q", result.Changes, want)
		}
		if fileExists(notesFile(repo)) {
			t.Fatal("migration preview wrote the sidecar")
		}
	})

	t.Run("retrofit", func(t *testing.T) {
		t.Parallel()
		repo := seedNonStandardRepo(t)

		svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
		result, err := svc.Retrofit(RetrofitInput{})
		if err != nil {
			t.Fatalf("retrofit dry run: %v", err)
		}
		if !slices.Contains(result.Changes, want) {
			t.Fatalf("changes = %v, want one entry %q", result.Changes, want)
		}
		if fileExists(notesFile(repo)) {
			t.Fatal("retrofit preview wrote the sidecar")
		}
	})
}

// An existing sidecar is not a pending change: reporting one would promise a
// write that no-clobber will never make.
func TestInitPreviewOmitsNotesWhenPresent(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, notesFile(repo), "# Repository Notes\n")

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(false)
	if err != nil {
		t.Fatalf("init dry run: %v", err)
	}
	for _, change := range result.Changes {
		if strings.Contains(change, notesFileName) {
			t.Fatalf("preview proposed %q over an existing sidecar", change)
		}
	}
}

// unsafeNotesDestinations are the destinations init must refuse rather than
// write through, each with the extra proof its shape needs after an apply.
var unsafeNotesDestinations = []struct {
	name       string
	plant      func(t *testing.T, repo string)
	afterApply func(t *testing.T, repo string)
}{
	{
		name: "symlink",
		// The target does not exist, so a stat-based no-clobber check would call
		// the destination absent and write through the link.
		plant: func(t *testing.T, repo string) {
			if err := os.Symlink(filepath.Join(repo, "outside.md"), notesFile(repo)); err != nil {
				t.Fatalf("plant symlink: %v", err)
			}
		},
		afterApply: func(t *testing.T, repo string) {
			if fileExists(filepath.Join(repo, "outside.md")) {
				t.Fatal("apply wrote through the planted symlink")
			}
		},
	},
	{
		name: "directory",
		plant: func(t *testing.T, repo string) {
			if err := os.MkdirAll(notesFile(repo), 0o755); err != nil {
				t.Fatalf("plant directory: %v", err)
			}
		},
	},
	{
		name: "case alias",
		plant: func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(planningDirOf(repo), "notes.md"), "# lower-case sibling\n")
		},
	},
}

// seedUnsafeNotes builds a migratable repository holding one unusable sidecar
// destination.
func seedUnsafeNotes(t *testing.T, plant func(t *testing.T, repo string)) string {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n")
	plant(t, repo)
	return repo
}

func TestInitRefusesUnsafeNotesDestination(t *testing.T) {
	t.Parallel()

	for _, tc := range unsafeNotesDestinations {
		t.Run(tc.name+"/preview", func(t *testing.T) {
			t.Parallel()
			repo := seedUnsafeNotes(t, tc.plant)
			svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
			_, err := svc.Init(false)
			assertPathBlocked(t, err)
		})
		t.Run(tc.name+"/apply", func(t *testing.T) {
			t.Parallel()
			repo := seedUnsafeNotes(t, tc.plant)
			svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
			_, err := svc.Init(true)
			assertPathBlocked(t, err)
			if tc.afterApply != nil {
				tc.afterApply(t, repo)
			}
		})
	}
}

func assertPathBlocked(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected refusal, got success")
	}
	if got := MachineFailureFor(err).Code; got != MachineCodePathBlocked {
		t.Fatalf("error code = %q, want %q (error: %v)", got, MachineCodePathBlocked, err)
	}
	if !strings.Contains(err.Error(), notesFileName) {
		t.Fatalf("refusal does not name the sidecar: %v", err)
	}
}

// The destination follows the configured planning directory, so a repository
// that keeps planning elsewhere gets its sidecar there and never at the root
// default.
func TestNotesDestinationFollowsConfiguredPlanningDir(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: docs/specs\nplanning_dir: docs/planning\n")

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(true); err != nil {
		t.Fatalf("init: %v", err)
	}
	if got := readFileString(t, filepath.Join(repo, "docs", "planning", notesFileName)); got != starterNotes() {
		t.Fatalf("configured-planning sidecar = %q, want the template", got)
	}
	if fileExists(notesFile(repo)) {
		t.Fatal("init wrote the sidecar at the hard-coded default planning path")
	}
}

// Classification decides the destination is absent; creation happens after. A
// symlink planted in that window must not be written through, which only an
// exclusive create guarantees — a re-stat would see the link's absent target and
// publish the template outside the planning directory.
func TestNotesTemplateDoesNotFollowALinkPlantedAfterClassification(t *testing.T) {
	repo := seedFixtureRepo(t)
	outside := filepath.Join(repo, "outside.md")
	testHookBeforeNotesCreate = func(path string) {
		testHookBeforeNotesCreate = nil
		if err := os.Symlink(outside, path); err != nil {
			t.Fatalf("plant symlink: %v", err)
		}
	}
	t.Cleanup(func() { testHookBeforeNotesCreate = nil })

	if err := ensureNotesTemplate(repo, planningDirOf(repo)); err != nil {
		t.Fatalf("ensure notes template: %v", err)
	}
	if fileExists(outside) {
		t.Fatal("template was written through the planted symlink")
	}
}

func TestNotesExtractionCandidatePreservesTextAndOrder(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		notes []string
	}{
		{"single", []string{"Bootstrapping until the CLI exists."}},
		{"multiple", []string{"first note", "second note", "third note"}},
		{"multiline", []string{"line one\nline two\n\nline four", "trailing note"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := seedFixtureRepo(t)

			candidate, err := notesExtractionCandidate(repo, planningDirOf(repo), tc.notes)
			if err != nil {
				t.Fatalf("extraction candidate: %v", err)
			}
			body := string(candidate)
			if !strings.HasPrefix(body, starterNotes()) {
				t.Fatalf("candidate does not start with the ordinary template:\n%s", body)
			}
			if !strings.Contains(body, notesImportedHeading) {
				t.Fatalf("candidate has no labelled imported section:\n%s", body)
			}
			previous := -1
			for i, note := range tc.notes {
				at := strings.Index(body, note)
				if at < 0 {
					t.Fatalf("note %d text was not preserved exactly:\n%s", i, body)
				}
				if at <= previous {
					t.Fatalf("note %d appears out of order:\n%s", i, body)
				}
				previous = at
			}
			if fileExists(notesFile(repo)) {
				t.Fatal("building a candidate wrote the sidecar")
			}
		})
	}
}

// Empty continuation notes need no extraction, so asking for one is a caller
// mistake rather than an empty section.
func TestNotesExtractionCandidateRejectsEmptyNotes(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)

	_, err := notesExtractionCandidate(repo, planningDirOf(repo), nil)
	if err == nil {
		t.Fatal("expected refusal for an empty note list")
	}
	if got := MachineFailureFor(err).Code; got != MachineCodeInvalidArguments {
		t.Fatalf("error code = %q, want %q (error: %v)", got, MachineCodeInvalidArguments, err)
	}
}

// Extraction never appends to or replaces an existing sidecar: that merge is the
// operator's, followed by dropping the notes instead.
func TestNotesExtractionCandidateRefusesExistingNotes(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	human := "# Repository Notes\n\nAlready mine.\n"
	writeFile(t, notesFile(repo), human)

	_, err := notesExtractionCandidate(repo, planningDirOf(repo), []string{"legacy note"})
	if err == nil {
		t.Fatal("expected refusal over an existing sidecar")
	}
	if got := MachineFailureFor(err).Code; got != MachineCodeDestinationExists {
		t.Fatalf("error code = %q, want %q (error: %v)", got, MachineCodeDestinationExists, err)
	}
	if got := readFileString(t, notesFile(repo)); got != human {
		t.Fatalf("refused extraction still changed the sidecar: %q", got)
	}
}

func TestNotesExtractionCandidateRefusesUnsafeDestination(t *testing.T) {
	t.Parallel()

	repo := seedUnsafeNotes(t, unsafeNotesDestinations[0].plant)

	_, err := notesExtractionCandidate(repo, planningDirOf(repo), []string{"legacy note"})
	assertPathBlocked(t, err)
}
