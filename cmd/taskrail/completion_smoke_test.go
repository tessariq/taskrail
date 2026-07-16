package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompleteSpecVersionArg verifies `spec show`/`spec activate` argument
// completion suggests the versioned specs under specs/ via cobra's __complete
// hook, and never file completion.
func TestCompleteSpecVersionArg(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"), []byte("# v0.2.0\n\n## Summary\n\nNext.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	for _, sub := range []string{"show", "activate"} {
		out, err := runRoot(t, "__complete", "spec", sub, "")
		if err != nil {
			t.Fatalf("__complete spec %s: %v (output %q)", sub, err, out)
		}
		if !strings.Contains(out, "v0.1.0") || !strings.Contains(out, "v0.2.0") {
			t.Fatalf("spec %s completion omits a version: %q", sub, out)
		}
		if !strings.Contains(out, "ShellCompDirectiveNoFileComp") {
			t.Fatalf("spec %s completion should suppress file completion: %q", sub, out)
		}
	}
}

// TestCompleteSpecRefPathPhase verifies `task new --spec-ref` first suggests spec
// paths with a trailing '#' and keeps the shell on the word (NoSpace) so the
// anchor phase can follow.
func TestCompleteSpecRefPathPhase(t *testing.T) {
	setupRepo(t)
	out, err := runRoot(t, "__complete", "task", "new", "--spec-ref", "")
	if err != nil {
		t.Fatalf("__complete --spec-ref: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "specs/v0.1.0.md#") {
		t.Fatalf("spec-ref path phase omits spec path: %q", out)
	}
	if !strings.Contains(out, "ShellCompDirectiveNoSpace") {
		t.Fatalf("spec-ref path phase should suppress the trailing space: %q", out)
	}
}

// TestCompleteAreaSuggestsActiveSpecAnchors verifies `task new --area` completes to
// the active spec's bare anchors — the same set validation accepts — and never
// falls back to file completion.
func TestCompleteAreaSuggestsActiveSpecAnchors(t *testing.T) {
	setupRepo(t)
	out, err := runRoot(t, "__complete", "task", "new", "--area", "")
	if err != nil {
		t.Fatalf("__complete --area: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "summary") {
		t.Fatalf("area completion omits an active-spec anchor: %q", out)
	}
	if !strings.Contains(out, "ShellCompDirectiveNoFileComp") {
		t.Fatalf("area completion should suppress file completion: %q", out)
	}
}

// TestCompleteSpecRefAnchorPhaseAuthorable is the end-to-end acceptance: an anchor
// suggested by --spec-ref completion authors a task whose spec_ref passes
// validate, proving completion reuses the real anchor slug rule.
func TestCompleteSpecRefAnchorPhaseAuthorable(t *testing.T) {
	root := setupRepo(t)
	out, err := runRoot(t, "__complete", "task", "new", "--spec-ref", "specs/v0.1.0.md#")
	if err != nil {
		t.Fatalf("__complete --spec-ref anchor: %v (output %q)", err, out)
	}

	var ref string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "specs/v0.1.0.md#") {
			ref = strings.TrimSpace(strings.SplitN(line, "\t", 2)[0])
			break
		}
	}
	if ref == "" {
		t.Fatalf("no anchor candidate suggested: %q", out)
	}

	writeTaskSpecRef(t, root, "T-901", ref)
	if vout, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate against completed spec_ref %q: %v (output %q)", ref, err, vout)
	}
}
