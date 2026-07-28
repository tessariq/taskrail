package taskrail

import (
	"testing"
)

const newerLayoutMarker = "layout_version: 999\nspecs_dir: specs\nplanning_dir: planning\n"

// The layout_version refusal must live on the shared load path every command
// reaches, not only in init, so an older binary cannot read-modify-write a layout
// it does not understand (specs/v0.4.0.md#layout-compatibility-beyond-init).
func TestLayoutLoadRejectsNewerLayoutVersion(t *testing.T) {
	t.Parallel()

	loaders := map[string]func(string) error{
		"DiscoverPaths": func(repo string) error { _, err := DiscoverPaths(repo); return err },
		"NewService":    func(repo string) error { _, err := NewService(repo); return err },
	}
	for name, load := range loaders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := seedFixtureRepo(t)
			writeFile(t, markerFile(repo), newerLayoutMarker)

			err := load(repo)
			if err == nil {
				t.Fatal("expected error for newer-than-supported layout_version")
			}
			want := "repository layout_version 999 is newer than supported 1; upgrade taskrail"
			if err.Error() != want {
				t.Fatalf("error = %q, want %q", err.Error(), want)
			}
		})
	}
}

// An equal or older layout_version must be unaffected: older still migrates
// through init, and every current repository behaves exactly as before.
func TestDiscoverPathsAcceptsCurrentAndOlderLayoutVersion(t *testing.T) {
	t.Parallel()

	for name, marker := range map[string]string{
		"current": "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n",
		"older":   "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := seedFixtureRepo(t)
			writeFile(t, markerFile(repo), marker)

			paths, err := DiscoverPaths(repo)
			if err != nil {
				t.Fatalf("discover paths: %v", err)
			}
			assertDefaultLayout(t, repo, paths)
		})
	}
}
