package taskrail

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// strayTaskContent is a task file whose id disagrees with its filename (the
// drift repair heals). Renames that would land on it must refuse rather than
// clobber it, so its content doubles as the survival marker the tests assert on.
const strayTaskContent = "---\nid: T-900\ntitle: Precious\nstatus: todo\npriority: low\nspec_ref: specs/v0.1.0.md#summary\ndependencies: []\nupdated_at: \"2026-01-01T00:00:00Z\"\n---\n\n# PRECIOUS DATA MUST SURVIVE\n"

func renameFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Dependent", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))
	return svc, repo
}

func readBytes(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	return string(data)
}

func taskDeps(t *testing.T, svc *Service, id string) []string {
	t.Helper()
	tasks, err := svc.loadTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	task, ok := taskByID(tasks, id)
	if !ok {
		t.Fatalf("task %s not found", id)
	}
	return task.Frontmatter.Dependencies
}

func TestRenameTaskReslugsIDFilenameAndInboundDeps(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "Base Widget"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !res.Applied {
		t.Fatalf("expected applied rename, got %+v", res)
	}
	if res.NewID != "T-001-base-widget" {
		t.Fatalf("new id = %q, want T-001-base-widget", res.NewID)
	}

	if fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
		t.Fatal("old task file still present")
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001-base-widget.md")) {
		t.Fatal("renamed task file missing")
	}

	if deps := taskDeps(t, svc, "T-002"); len(deps) != 1 || deps[0] != "T-001-base-widget" {
		t.Fatalf("inbound dependency not rewritten: %v", deps)
	}

	// A dependency_ref change is reported for the inbound task.
	var sawDepChange bool
	for _, ch := range res.Changes {
		if ch.Kind == "dependency_ref" && ch.TaskID == "T-002" && ch.From == "T-001" && ch.To == "T-001-base-widget" {
			sawDepChange = true
		}
	}
	if !sawDepChange {
		t.Fatalf("missing dependency_ref change: %+v", res.Changes)
	}

	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate after rename: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

func TestRenameTaskTitleDerivesSlug(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Title: "Base Widget"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if res.NewID != "T-001-base-widget" {
		t.Fatalf("new id = %q, want T-001-base-widget", res.NewID)
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001-base-widget.md")) {
		t.Fatal("renamed task file missing")
	}
	// --title is only a slug source; it must not rewrite the frontmatter title.
	tasks, _ := svc.loadTasks()
	task, _ := taskByID(tasks, "T-001-base-widget")
	if task.Frontmatter.Title != "Base" {
		t.Fatalf("frontmatter title changed to %q, want unchanged 'Base'", task.Frontmatter.Title)
	}
}

// TestRenameTaskCapsTitleDerivedSlugOnly pins the v0.4.0 cap on the rename path
// too (T-126): a `--title`-derived slug is bounded near slugMaxLen on a hyphen
// boundary exactly as `task new --title` bounds it, so one title yields one
// length whichever command wrote it. An explicit `--slug` stays the operator's
// verbatim curation. The capped id is the id everywhere the rename writes one:
// frontmatter, filename, and every inbound dependency edge.
func TestRenameTaskCapsTitleDerivedSlugOnly(t *testing.T) {
	t.Parallel()

	longTitle := "Cap the title derived slug length at roughly fifty characters boundary aware"
	longSlug := "an-intentionally-long-but-curated-slug-the-operator-owns-verbatim"

	cases := []struct {
		name   string
		input  RenameTaskInput
		wantID string
	}{
		{
			name:   "title-derived slug is capped on a hyphen boundary",
			input:  RenameTaskInput{OldID: "T-001", Title: longTitle},
			wantID: "T-001-cap-the-title-derived-slug-length-at-roughly-fifty",
		},
		{
			name:   "explicit slug is not capped",
			input:  RenameTaskInput{OldID: "T-001", Slug: longSlug},
			wantID: "T-001-" + longSlug,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := renameFixture(t)

			res, err := svc.RenameTask(tc.input)
			if err != nil {
				t.Fatalf("rename: %v", err)
			}
			if res.NewID != tc.wantID {
				t.Fatalf("new id = %q, want %q", res.NewID, tc.wantID)
			}
			if len(res.Warnings) != 0 {
				t.Fatalf("unexpected warnings: %+v", res.Warnings)
			}
			// The id and filename are two encodings of one identifier.
			if !fileExists(filepath.Join(repo, "planning", "tasks", tc.wantID+".md")) {
				t.Fatalf("renamed task file %s.md missing", tc.wantID)
			}
			if fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
				t.Fatal("old task file still present")
			}
			if deps := taskDeps(t, svc, "T-002"); !slices.Equal(deps, []string{tc.wantID}) {
				t.Fatalf("inbound dependency = %v, want [%s]", deps, tc.wantID)
			}
			if v, err := svc.Validate(); err != nil || !v.Valid {
				t.Fatalf("validate after rename: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
			}
		})
	}
}

func TestRenameTaskPreservesNumericPrefix(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Numbered", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "new-slug"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if res.NewID != "T-042-new-slug" {
		t.Fatalf("new id = %q, want T-042-new-slug", res.NewID)
	}
}

func TestRenameTaskCollisionMakesNoPartialChange(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-alpha", "Alpha", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-001-beta", "Beta", "todo", "high", "specs/v0.1.0.md#summary", nil)
	// Impossible numeric-prefix collision guard: two T-001 ids can only exist in a
	// crafted fixture, but the collision path must still refuse cleanly.
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001-alpha", Slug: "beta"}); err == nil {
		t.Fatal("expected collision error")
	}
	// No partial change: the source file is untouched and no target overwrite happened.
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001-alpha.md")) {
		t.Fatal("source file lost on collision")
	}
}

func TestRenameTaskDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base-widget", DryRun: true})
	if err != nil {
		t.Fatalf("rename dry run: %v", err)
	}
	if res.Applied {
		t.Fatal("dry run must not apply")
	}
	if len(res.Changes) == 0 {
		t.Fatal("dry run should still report the planned change set")
	}
	// Nothing on disk moved and no inbound edit landed.
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
		t.Fatal("dry run renamed the file")
	}
	if fileExists(filepath.Join(repo, "planning", "tasks", "T-001-base-widget.md")) {
		t.Fatal("dry run wrote the target file")
	}
	if deps := taskDeps(t, svc, "T-002"); len(deps) != 1 || deps[0] != "T-001" {
		t.Fatalf("dry run edited inbound deps: %v", deps)
	}
}

// driftedTask writes a task whose filename disagrees with its frontmatter id,
// the `filename must be <id>.md` violation an operator re-slugs to heal. It
// returns the frontmatter id so the caller renames by the id RenameTask resolves.
func driftedTask(t *testing.T, repo, id, driftedBase string) string {
	t.Helper()
	writeTask(t, repo, id, "Drifted", "todo", "high", "specs/v0.1.0.md#summary", nil)
	src := filepath.Join(repo, "planning", "tasks", id+".md")
	dst := filepath.Join(repo, "planning", "tasks", driftedBase+".md")
	if err := os.Rename(src, dst); err != nil {
		t.Fatalf("drift file: %v", err)
	}
	return id
}

func TestRenameTaskDryRunValidationPreviewsPostApplyState(t *testing.T) {
	t.Parallel()

	// The reason to re-slug is often that the file/id are out of sync — here the
	// file is named for a slug the frontmatter id no longer carries, so the repo
	// is invalid now. A dry run must answer "would this heal it?", so its
	// validation reflects the state the rename would produce, not the one it
	// replaces.
	repo := seedFixtureRepo(t)
	id := driftedTask(t, repo, "T-042-old", "T-042-drifted")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	current, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if current.Valid {
		t.Fatal("expected the pre-rename repo to be invalid")
	}

	drifted := filepath.Join(repo, "planning", "tasks", "T-042-drifted.md")
	before := readBytes(t, drifted)
	result, err := svc.RenameTask(RenameTaskInput{OldID: id, Slug: "healed", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run rename: %v", err)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected dry run to preview a valid post-apply state, got %+v", result.Validation)
	}
	// Previewing must stay side-effect-free.
	if got := readBytes(t, drifted); got != before {
		t.Fatal("expected task file untouched on dry run")
	}

	// The preview is honest: applying really does yield that result.
	applied, err := svc.RenameTask(RenameTaskInput{OldID: id, Slug: "healed"})
	if err != nil {
		t.Fatalf("apply rename: %v", err)
	}
	if applied.Validation == nil || !applied.Validation.Valid {
		t.Fatalf("expected valid state after apply, got %+v", applied.Validation)
	}
}

func TestRenameTaskHealsDriftWhenFilenameIsRequestedDestination(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	id := driftedTask(t, repo, "T-042-old", "T-042-readable")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	result, err := svc.RenameTask(RenameTaskInput{OldID: id, Slug: "readable"})
	if err != nil {
		t.Fatalf("heal drift at requested destination: %v", err)
	}
	if result.NewID != "T-042-readable" {
		t.Fatalf("new id = %q, want T-042-readable", result.NewID)
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-042-readable.md")) {
		t.Fatal("healed task file missing")
	}
	if validation, err := svc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("validate after healing drift: valid=%v violations=%v err=%v", validation.Valid, validation.Violations, err)
	}
}

func TestRenameTaskDryRunPreviewKeepsUnrelatedViolations(t *testing.T) {
	t.Parallel()

	// The preview applies only the rename change set: a violation the rename does
	// not touch must still surface, so a dry run never reads as a clean bill of
	// health for the whole repo.
	repo := seedFixtureRepo(t)
	id := driftedTask(t, repo, "T-042-old", "T-042-drifted")
	writeTask(t, repo, "T-003", "Broken sibling", "todo", "medium", "specs/v0.9.9.md#gone", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	result, err := svc.RenameTask(RenameTaskInput{OldID: id, Slug: "healed", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run rename: %v", err)
	}
	if result.Validation == nil || result.Validation.Valid {
		t.Fatalf("expected the unrelated violation to survive the preview, got %+v", result.Validation)
	}
	found := false
	for _, v := range result.Validation.Violations {
		if strings.Contains(v, "T-003") {
			found = true
		}
		if strings.Contains(v, "T-042") {
			t.Fatalf("expected no violation for the re-slugged task, got %q", v)
		}
	}
	if !found {
		t.Fatalf("expected a T-003 violation, got %+v", result.Validation.Violations)
	}
}

func TestRenameTaskApplyRefusesInvalidPreviewWithoutWrites(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)
	writeTask(t, repo, "T-003", "Broken sibling", "todo", "medium", "specs/v0.9.9.md#gone", nil)

	tasksDir := filepath.Join(repo, "planning", "tasks")
	source := filepath.Join(tasksDir, "T-001.md")
	inbound := filepath.Join(tasksDir, "T-002.md")
	stateFile := filepath.Join(repo, "planning", "STATE.md")
	sourceBefore, inboundBefore, stateBefore := readBytes(t, source), readBytes(t, inbound), readBytes(t, stateFile)

	_, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base-widget"})
	if err == nil {
		t.Fatal("expected rename to reject an invalid post-apply preview")
	}
	if !strings.Contains(err.Error(), "T-003") {
		t.Fatalf("expected error to report the surviving violation, got %v", err)
	}
	if got := readBytes(t, source); got != sourceBefore {
		t.Fatalf("source task changed after rejected rename:\n%s", got)
	}
	if got := readBytes(t, inbound); got != inboundBefore {
		t.Fatalf("inbound task changed after rejected rename:\n%s", got)
	}
	if got := readBytes(t, stateFile); got != stateBefore {
		t.Fatalf("STATE.md changed after rejected rename:\n%s", got)
	}
	if fileExists(filepath.Join(tasksDir, "T-001-base-widget.md")) {
		t.Fatal("rejected rename created the target file")
	}
}

// TestRenameTaskDryRunPreviewsCurrentTaskPointer pins that the preview repoints
// the current_task pointer for the active task it renames. Without it, the
// pending id and the still-old current_task disagree and the preview would report
// invalid (state current_task does not match in_progress task) — the inverse of
// what the operator asked.
func TestRenameTaskDryRunPreviewsCurrentTaskPointer(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-003-active", "Active", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-003-active", "Active", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	before := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))
	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-003-active", Slug: "running", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run rename: %v", err)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected the current_task pointer to be previewed valid, got %+v", result.Validation)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != before {
		t.Fatal("dry run wrote STATE.md")
	}
}

func TestRenameTaskRequiresExactlyOneSelector(t *testing.T) {
	t.Parallel()
	svc, _ := renameFixture(t)

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001"}); err == nil {
		t.Fatal("expected error when neither --slug nor --title is given")
	}
	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "a", Title: "b"}); err == nil {
		t.Fatal("expected error when both --slug and --title are given")
	}
}

func TestRenameTaskMissingTaskErrors(t *testing.T) {
	t.Parallel()
	svc, _ := renameFixture(t)
	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-999", Slug: "x"}); err == nil {
		t.Fatal("expected error for unknown task id")
	}
}

func TestRenameTaskRefusesWhenDestinationFileExists(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	// A stray file already occupies the rename's target path; a plain os.Rename
	// would silently clobber it.
	stray := filepath.Join(repo, "planning", "tasks", "T-001-widget.md")
	writeFile(t, stray, strayTaskContent)

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "Widget"}); err == nil {
		t.Fatal("expected error when the target file already exists")
	}
	if got := readBytes(t, stray); got != strayTaskContent {
		t.Fatalf("stray file overwritten:\n%s", got)
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
		t.Fatal("source file lost on destination collision")
	}
}

func TestRenameTaskReprojectsStateBody(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-003-active", "Active", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-003-active", "Active", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-003-active", Slug: "running"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	// The rendered STATE.md body — not just the frontmatter — must reflect the new
	// id, so the Current Focus section stays consistent with the projection.
	if !strings.Contains(state.Body, "T-003-running") {
		t.Fatalf("STATE.md body not re-projected to new id:\n%s", state.Body)
	}
	if strings.Contains(state.Body, "`T-003-active`") {
		t.Fatalf("STATE.md body still shows the old id:\n%s", state.Body)
	}
}

// TestRenameTaskDeSlugsOnEmptyDerivedSlug pins the symmetric counterpart of
// creation's bare-id fallback: a selector that normalizes to nothing strips the
// slug rather than failing, so an operator can undo a bad slug. The de-slug is a
// full rename — inbound edges follow — and it warns for the same reason creation
// does.
func TestRenameTaskDeSlugsOnEmptyDerivedSlug(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Slugged", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-043", "Dependent", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-042-old-slug"})
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "!!!"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if result.NewID != "T-042" {
		t.Fatalf("expected de-slug to bare T-042, got %s", result.NewID)
	}
	if fileExists(filepath.Join(repo, "planning", "tasks", "T-042-old-slug.md")) {
		t.Fatal("old slugged file still present after de-slug")
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-042.md")) {
		t.Fatal("expected bare T-042.md after de-slug")
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "empty_derived_slug" {
		t.Fatalf("expected one empty-slug warning, got %v", result.Warnings)
	}
	if !strings.Contains(result.Warnings[0].Message, "!!!") || !strings.Contains(result.Warnings[0].Message, "T-042") {
		t.Fatalf("warning must name the source and the bare id, got %q", result.Warnings[0].Message)
	}

	// De-slugging is a rename like any other: inbound edges must follow the id.
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	dependent, ok := taskByID(tasks, "T-043")
	if !ok {
		t.Fatal("expected T-043 in tasks")
	}
	if !slices.Contains(dependent.Frontmatter.Dependencies, "T-042") {
		t.Fatalf("inbound dependency not repointed: %v", dependent.Frontmatter.Dependencies)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid state after de-slug, got %+v", result.Validation)
	}
}

// TestRenameTaskRewritesBodyHeading pins the last place the old id survived a
// rename: the body's `# <id> <title>` H1. Frontmatter, filename and inbound edges
// already followed the new id, so a stale heading left the file naming two
// different ids. The title text is not part of the identifier and must survive
// untouched — rename re-slugs, it never retitles.
func TestRenameTaskRewritesBodyHeading(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Slugged", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "new-slug"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	body := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-042-new-slug.md"))
	if !strings.Contains(body, "# T-042-new-slug Slugged") {
		t.Fatalf("body heading not repointed to the new id:\n%s", body)
	}
	if strings.Contains(body, "T-042-old-slug") {
		t.Fatalf("body still names the old id:\n%s", body)
	}

	// The heading edit is part of the reported change set, so --dry-run and --json
	// disclose it instead of it happening invisibly.
	change, ok := bodyHeadingChange(result.Changes)
	if !ok {
		t.Fatalf("missing body_heading change: %+v", result.Changes)
	}
	if change.From != "# T-042-old-slug Slugged" || change.To != "# T-042-new-slug Slugged" {
		t.Fatalf("unexpected body_heading change: %+v", change)
	}
}

// bodyHeadingChange returns the rename's body_heading change, which the change set
// carries only when a heading was actually rewritten.
func bodyHeadingChange(changes []RenameChange) (RenameChange, bool) {
	for _, change := range changes {
		if change.Kind == "body_heading" {
			return change, true
		}
	}
	return RenameChange{}, false
}

// TestRenameBodyHeadingMatchesWholeIDToken guards the boundary the prefix check
// exists for: `T-1` is a string prefix of `T-10`, so a bare HasPrefix would rewrite
// the heading of a task the rename is not about. The id token has to end at a space
// or the end of the line to count.
func TestRenameBodyHeadingMatchesWholeIDToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		oldID   string
		want    string
		changed bool
	}{
		{
			name:    "id followed by a title",
			body:    "# T-1 Some title\n\n## Description\n",
			oldID:   "T-1",
			want:    "# T-9 Some title\n\n## Description\n",
			changed: true,
		},
		{
			name:    "id alone on the heading line",
			body:    "# T-1\n\n## Description\n",
			oldID:   "T-1",
			want:    "# T-9\n\n## Description\n",
			changed: true,
		},
		{
			name:    "longer id merely starting with the old one",
			body:    "# T-10 Other task\n\n## Description\n",
			oldID:   "T-1",
			want:    "# T-10 Other task\n\n## Description\n",
			changed: false,
		},
		{
			name:    "slug suffix is part of the id token",
			body:    "# T-1-slug Other task\n",
			oldID:   "T-1",
			want:    "# T-1-slug Other task\n",
			changed: false,
		},
		{
			name:    "heading not at the start of the body",
			body:    "Preamble\n\n# T-1 Some title\n",
			oldID:   "T-1",
			want:    "Preamble\n\n# T-1 Some title\n",
			changed: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, changed := renameBodyHeading(tc.body, tc.oldID, "T-9")
			if changed != tc.changed {
				t.Fatalf("changed = %v, want %v (body %q)", changed, tc.changed, tc.body)
			}
			if got != tc.want {
				t.Fatalf("renameBodyHeading(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

// TestRenameTaskLeavesUnrelatedBodyHeadingAlone keeps the rewrite conservative: a
// hand-authored body whose H1 does not lead with the task's id is content, not an
// identifier encoding, so the rename must not touch it — and must not claim it did.
func TestRenameTaskLeavesUnrelatedBodyHeadingAlone(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Slugged", "todo", "high", "specs/v0.1.0.md#summary", nil)
	path := filepath.Join(repo, "planning", "tasks", "T-042-old-slug.md")
	original := readBytes(t, path)
	custom := strings.Replace(original, "# T-042-old-slug Slugged", "# A hand-written heading", 1)
	if custom == original {
		t.Fatal("fixture heading not replaced; the test would prove nothing")
	}
	writeFile(t, path, custom)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "new-slug"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	body := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-042-new-slug.md"))
	if !strings.Contains(body, "# A hand-written heading") {
		t.Fatalf("hand-written heading was rewritten:\n%s", body)
	}
	if change, ok := bodyHeadingChange(result.Changes); ok {
		t.Fatalf("reported a body_heading change that did not happen: %+v", change)
	}
}

// TestRenameTaskDryRunLeavesBodyHeading keeps the heading inside the dry-run
// contract: previewed, never written.
func TestRenameTaskDryRunLeavesBodyHeading(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Slugged", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "new-slug", DryRun: true})
	if err != nil {
		t.Fatalf("rename dry run: %v", err)
	}
	if _, ok := bodyHeadingChange(result.Changes); !ok {
		t.Fatalf("dry run should preview the heading edit: %+v", result.Changes)
	}
	body := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-042-old-slug.md"))
	if !strings.Contains(body, "# T-042-old-slug Slugged") {
		t.Fatalf("dry run rewrote the heading:\n%s", body)
	}
}

// TestRenameTaskDeSlugDryRunWritesNothing keeps the de-slug inside the dry-run
// contract: a preview of a slug-stripping rename still reports the change set and
// the warning, but the task keeps its slugged filename until the run is repeated
// for real.
func TestRenameTaskDeSlugDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Slugged", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "!!!", DryRun: true})
	if err != nil {
		t.Fatalf("rename dry run: %v", err)
	}
	if result.Applied {
		t.Fatal("dry run must not apply the de-slug")
	}
	if result.NewID != "T-042" {
		t.Fatalf("expected planned de-slug to T-042, got %s", result.NewID)
	}
	if len(result.Changes) == 0 {
		t.Fatal("dry run should still report the planned change set")
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "empty_derived_slug" {
		t.Fatalf("expected one empty-slug warning on the preview, got %v", result.Warnings)
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-042-old-slug.md")) {
		t.Fatal("dry run moved the slugged file")
	}
	if fileExists(filepath.Join(repo, "planning", "tasks", "T-042.md")) {
		t.Fatal("dry run wrote the bare-id target")
	}
}

// TestRenameTaskRejectsEmptySlugOnAlreadyBareTask keeps the no-op guard: with no
// slug to strip, an empty-normalizing selector cannot change the id, so it fails
// instead of reporting a rename that did nothing.
func TestRenameTaskRejectsEmptySlugOnAlreadyBareTask(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042", "Bare", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-042", Slug: "!!!"}); err == nil {
		t.Fatal("expected error when de-slugging an already-bare task")
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-042.md")) {
		t.Fatal("bare task file disturbed by the rejected rename")
	}
}

func TestRenameTaskRewritesAllInboundDependents(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Dep A", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	writeTask(t, repo, "T-003", "Dep B", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	for _, id := range []string{"T-002", "T-003"} {
		if deps := taskDeps(t, svc, id); len(deps) != 1 || deps[0] != "T-001-base" {
			t.Fatalf("%s inbound dep not rewritten: %v", id, deps)
		}
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

func TestRenameTaskRollsBackWhenInboundWriteFails(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)
	tasksDir := filepath.Join(repo, "planning", "tasks")
	source := filepath.Join(tasksDir, "T-001.md")
	inbound := filepath.Join(tasksDir, "T-002.md")
	stateFile := filepath.Join(repo, "planning", "STATE.md")

	// Fail the rename *after* the file move and the target rewrite have landed:
	// the inbound dependent is the third coupled write, so this is the exact
	// partial-write window the atomicity contract has to close.
	requireReadOnlyFileBlocksWrites(t, inbound)
	sourceBefore, inboundBefore, stateBefore := readBytes(t, source), readBytes(t, inbound), readBytes(t, stateFile)

	_, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"})
	if err == nil {
		t.Fatal("expected rename to fail on the unwritable inbound task file")
	}
	assertNoRootLeak(t, repo, err)
	// A permission-denied write never truncates, so restoring it is a no-op: the
	// rollback fully succeeded and the operator must be told the tree is clean,
	// not sent to repair for drift that does not exist.
	if !strings.Contains(err.Error(), "no changes were applied") {
		t.Fatalf("clean rollback reported as failed: %v", err)
	}

	if fileExists(filepath.Join(tasksDir, "T-001-base.md")) {
		t.Fatal("renamed file left behind after rollback")
	}
	if got := readBytes(t, source); got != sourceBefore {
		t.Fatalf("source task file not restored:\n%s", got)
	}
	if got := readBytes(t, inbound); got != inboundBefore {
		t.Fatalf("inbound task file changed:\n%s", got)
	}
	if got := readBytes(t, stateFile); got != stateBefore {
		t.Fatalf("STATE.md changed despite the failed rename:\n%s", got)
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("tree not valid after rollback: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

func TestRenameFailureReportsRollbackOutcome(t *testing.T) {
	t.Parallel()
	cause := errors.New("write task file T-002.md: permission denied")

	rolledBack := renameFailure("T-001", "T-001-base", cause, nil)
	if !errors.Is(rolledBack, cause) {
		t.Fatalf("cause not wrapped: %v", rolledBack)
	}
	if !strings.Contains(rolledBack.Error(), "no changes were applied") {
		t.Fatalf("clean rollback must say the tree is unchanged: %v", rolledBack)
	}
	if strings.Contains(rolledBack.Error(), "repair") {
		t.Fatalf("clean rollback must not send the operator to repair: %v", rolledBack)
	}

	// Rollback itself failed, so the tree may be half renamed: the error is the
	// operator's only signal and must name the reconcile commands.
	stuck := renameFailure("T-001", "T-001-base", cause, errors.New("restore T-001.md: read-only file system"))
	if !errors.Is(stuck, cause) {
		t.Fatalf("cause not wrapped: %v", stuck)
	}
	for _, want := range []string{"taskrail validate", "taskrail repair --apply", "restore T-001.md"} {
		if !strings.Contains(stuck.Error(), want) {
			t.Fatalf("rollback-failure error missing %q: %v", want, stuck)
		}
	}
}

func TestRenameTaskUpdatesCurrentTaskPointer(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-003-active", "Active", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-003-active", "Active", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-003-active", Slug: "running"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if res.NewID != "T-003-running" {
		t.Fatalf("new id = %q", res.NewID)
	}
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Frontmatter.CurrentTask != "T-003-running" {
		t.Fatalf("current_task = %q, want T-003-running", state.Frontmatter.CurrentTask)
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate after rename of active task: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}
