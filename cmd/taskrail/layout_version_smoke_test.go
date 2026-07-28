package main

import (
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// A marker recording a layout the running binary cannot model must be refused by
// every command that loads the layout — writers before any read-modify-write, and
// read-only reporters too, because a plausible-looking report against an
// unmodelled layout is worse than a refusal
// (specs/v0.4.0.md#layout-compatibility-beyond-init).
func TestCommandsRefuseNewerLayoutVersion(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "todo", "")
	if _, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("seeded repo must validate before the marker is bumped: %v", err)
	}
	marker := filepath.Join(root, ".taskrail", "config.yml")
	if err := os.WriteFile(marker, []byte("layout_version: 999\nspecs_dir: specs\nplanning_dir: planning\n"), 0o644); err != nil {
		t.Fatalf("write marker %s: %v", marker, err)
	}
	before := snapshotTree(t, root)

	const want = "repository layout_version 999 is newer than supported 1; upgrade taskrail"
	cases := map[string][]string{
		"validate": {"validate"},
		"status":   {"status"},
		"stats":    {"stats"},
		"coverage": {"coverage"},
		"next":     {"next"},
		"start":    {"start", "T-001"},
		"complete": {"complete", "T-001", "--note", "done"},
		"block":    {"block", "T-001", "--reason", "stuck"},
		"verify":   {"verify", "T-001", "--result", "pass", "--summary", "ok"},
		"repair":   {"repair", "--apply"},
		"init":     {"init"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := runRoot(t, args...)
			if err == nil {
				t.Fatalf("%s accepted a newer layout_version (output %q)", name, out)
			}
			if err.Error() != want {
				t.Fatalf("%s error = %q, want %q", name, err.Error(), want)
			}
		})
	}

	if after := snapshotTree(t, root); !maps.Equal(before, after) {
		t.Fatal("a refused command wrote to the repository")
	}
}

// The current layout_version stays untouched by the guard: the same commands run
// normally against a repository the binary does understand.
func TestCommandsAcceptCurrentLayoutVersion(t *testing.T) {
	setupRepo(t)

	for _, args := range [][]string{{"validate"}, {"status"}, {"stats"}, {"coverage"}} {
		if out, err := runRoot(t, args...); err != nil {
			t.Fatalf("%v on current layout: %v (output %q)", args, err, out)
		}
	}
}

// snapshotTree records every regular file's content so a test can assert a
// refused command left the repository byte-identical.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return files
}
