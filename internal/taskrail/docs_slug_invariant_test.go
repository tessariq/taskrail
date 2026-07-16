package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// slugInvariantHeading is the README subsection T-097 requires. The markers below
// are asserted within this section, not the whole file, so deleting the section
// fails the test even if a marker phrase survives elsewhere in the README.
const slugInvariantHeading = "### The slug-in-id invariant"

// readmeSection returns the body of the README subsection introduced by heading,
// up to the next same-or-higher-level heading. Empty if the heading is absent.
func readmeSection(readme, heading string) string {
	start := strings.Index(readme, heading)
	if start < 0 {
		return ""
	}
	body := readme[start+len(heading):]
	for _, marker := range []string{"\n## ", "\n### "} {
		if end := strings.Index(body, marker); end >= 0 {
			body = body[:end]
		}
	}
	return body
}

// TestReadmeDocumentsSlugInIdInvariant guards the T-097 requirement that the
// slug-in-id model is documented where operators author tasks, so the invariant
// is discoverable instead of learned by hitting a `validate` failure. It asserts
// the dedicated README section states the id/filename coupling, the `task new`/
// `task rename` slug flows, and the bare-`git mv` rename trap with its fix. It
// checks presence of the load-bearing phrases, not exact prose.
func TestReadmeDocumentsSlugInIdInvariant(t *testing.T) {
	section := readmeSection(readReadme(t), slugInvariantHeading)
	if section == "" {
		t.Fatalf("README.md missing %q section", slugInvariantHeading)
	}

	markers := []struct {
		phrase string
		why    string
	}{
		{`filename == "<id>.md"`, "the validate rule that couples id and filename"},
		{"two encodings", "id and filename are two encodings of one identifier"},
		{"task new", "the slug-on-create behavior"},
		{"task rename", "the sanctioned re-slug flow / trap fix"},
		{"git mv", "the trap: a bare git mv that only renames the file"},
	}
	for _, m := range markers {
		if !strings.Contains(section, m.phrase) {
			t.Errorf("%q section missing %q (%s)", slugInvariantHeading, m.phrase, m.why)
		}
	}
}

// TestReadmeQuotedValidateErrorMatchesRuntime ties the trap error text the README
// quotes to the message `Validate()` actually emits, so the two cannot drift
// silently: if validation's wording changes, the templated phrase the README
// documents no longer matches the runtime violation and this test fails. It
// mirrors the doc_skill_lists cross-check against an authoritative source rather
// than trusting a second hardcoded literal.
func TestReadmeQuotedValidateErrorMatchesRuntime(t *testing.T) {
	repo := seedFixtureRepo(t)
	// A bare `git mv` reproduces the trap: the file is renamed to add a slug but
	// the frontmatter id stays bare, so base != "<id>.md".
	const id = "T-777"
	writeTask(t, repo, id, "Trap", "todo", "high", "specs/v0.1.0.md#summary", nil)
	tasks := filepath.Join(repo, "planning", "tasks")
	if err := os.Rename(filepath.Join(tasks, id+".md"), filepath.Join(tasks, id+"-add-slug.md")); err != nil {
		t.Fatalf("rename fixture task: %v", err)
	}

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	runtime := ""
	for _, v := range result.Violations {
		if strings.Contains(v, "filename must be") {
			runtime = v
			break
		}
	}
	if runtime == "" {
		t.Fatalf("no filename violation from Validate; got %v", result.Violations)
	}

	// Abstract the concrete id back to the README's placeholder and require the
	// documented section to quote exactly that runtime wording.
	documented := strings.ReplaceAll(runtime, id, "<id>")
	section := readmeSection(readReadme(t), slugInvariantHeading)
	if !strings.Contains(section, documented) {
		t.Errorf("%q section does not quote the runtime validate error %q (as %q)", slugInvariantHeading, runtime, documented)
	}
}

func readReadme(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
