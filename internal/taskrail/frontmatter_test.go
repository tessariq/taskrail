package taskrail

import (
	"bytes"
	"testing"
)

// Files authored (or Git-checked-out) on Windows use CRLF line endings. The
// frontmatter parser must treat them the same as LF, otherwise every task and
// state file fails to parse on Windows and `taskrail validate` reports the repo
// invalid (regression caught by the OS-matrix CI in T-046/T-047).
func TestParseFrontmatterHandlesCRLF(t *testing.T) {
	crlf := "---\r\nid: T-001\r\ntitle: Example\r\n---\r\n\r\nBody line one\r\nBody line two\r\n"

	fm, body, err := parseFrontmatter[TaskFrontmatter]([]byte(crlf))
	if err != nil {
		t.Fatalf("parseFrontmatter on CRLF input: %v", err)
	}
	if fm.ID != "T-001" {
		t.Errorf("id: got %q, want %q", fm.ID, "T-001")
	}
	if fm.Title != "Example" {
		t.Errorf("title: got %q, want %q", fm.Title, "Example")
	}
	if want := "Body line one\nBody line two\n"; body != want {
		t.Errorf("body: got %q, want %q", body, want)
	}

	// Persistence guarantee: re-marshalling never re-introduces CR bytes, so a
	// CRLF file read then saved is normalized to LF on disk.
	out, err := marshalFrontmatter(fm, body)
	if err != nil {
		t.Fatalf("marshalFrontmatter: %v", err)
	}
	if bytes.ContainsRune(out, '\r') {
		t.Errorf("marshalFrontmatter output contains CR: %q", out)
	}
}

// CR-only line endings (classic Mac / some legacy tools) must parse too; the
// parser normalizes every ending, not just CRLF.
func TestParseFrontmatterHandlesLoneCR(t *testing.T) {
	cr := "---\rid: T-002\rtitle: Legacy\r---\r\rBody\r"

	fm, body, err := parseFrontmatter[TaskFrontmatter]([]byte(cr))
	if err != nil {
		t.Fatalf("parseFrontmatter on CR-only input: %v", err)
	}
	if fm.ID != "T-002" {
		t.Errorf("id: got %q, want %q", fm.ID, "T-002")
	}
	if want := "Body\n"; body != want {
		t.Errorf("body: got %q, want %q", body, want)
	}
}
