package taskrail

import (
	"os"
	"strings"
	"testing"
)

// Marker read errors name the file the same way the rest of Taskrail names
// files: repo-relative, so error text stays portable and does not leak the
// producer's absolute filesystem layout (T-136). This deliberately asserts
// nothing about the caller wording TestLayoutMarkerReadErrorsKeepCallerWording
// pins, so the two contracts stay free to change independently.
func TestLayoutMarkerReadErrorsReportRepoRelativePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		read func(string) error
		seed func(*testing.T, string)
	}{
		{
			name: "loadLayoutConfig/parse",
			read: func(repo string) error { _, err := loadLayoutConfig(repo); return err },
			seed: seedUnparsableMarker,
		},
		{
			name: "loadLayoutConfig/read",
			read: func(repo string) error { _, err := loadLayoutConfig(repo); return err },
			seed: seedUnreadableMarker,
		},
		{
			name: "readMarker/parse",
			read: func(repo string) error { _, _, err := readMarker(repo); return err },
			seed: seedUnparsableMarker,
		},
		{
			name: "readMarker/read",
			read: func(repo string) error { _, _, err := readMarker(repo); return err },
			seed: seedUnreadableMarker,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := initGitRepo(t)
			tc.seed(t, repo)

			err := tc.read(repo)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			const wantPath = ".taskrail/config.yml"
			if !strings.Contains(err.Error(), wantPath) {
				t.Fatalf("error = %q, want it to name %q", err.Error(), wantPath)
			}
			// An absolute path ends in wantPath too, so the check above passes
			// either way; the repo root's absence is what pins the form.
			if strings.Contains(err.Error(), repo) {
				t.Fatalf("error = %q, want no absolute repo path %q", err.Error(), repo)
			}
		})
	}
}

func seedUnparsableMarker(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, markerFile(repo), "layout_version: [not-an-int\n")
}

// A directory where the marker file belongs makes os.ReadFile fail with
// something other than ErrNotExist, which is the only way to reach the
// read-error branch rather than the absent-marker branch.
func seedUnreadableMarker(t *testing.T, repo string) {
	t.Helper()
	if err := os.MkdirAll(markerFile(repo), 0o755); err != nil {
		t.Fatalf("mkdir marker path: %v", err)
	}
}

// The two marker readers share a read/unmarshal/version-guard sequence but
// deliberately report it with different adopter-facing wording: discovery calls
// the file a "layout config", init calls it a "layout marker". A fold of the
// shared sequence must not collapse that wording (T-131).
func TestLayoutMarkerReadErrorsKeepCallerWording(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		read func(string) error
		seed func(*testing.T, string)
		want string
	}{
		{
			name: "loadLayoutConfig/parse",
			read: func(repo string) error { _, err := loadLayoutConfig(repo); return err },
			seed: seedUnparsableMarker,
			want: "parse layout config",
		},
		{
			name: "loadLayoutConfig/read",
			read: func(repo string) error { _, err := loadLayoutConfig(repo); return err },
			seed: seedUnreadableMarker,
			want: "read layout config",
		},
		{
			name: "readMarker/parse",
			read: func(repo string) error { _, _, err := readMarker(repo); return err },
			seed: seedUnparsableMarker,
			want: "parse layout marker",
		},
		{
			name: "readMarker/read",
			read: func(repo string) error { _, _, err := readMarker(repo); return err },
			seed: seedUnreadableMarker,
			want: "read layout marker",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := initGitRepo(t)
			tc.seed(t, repo)

			err := tc.read(repo)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.HasPrefix(err.Error(), tc.want+" ") {
				t.Fatalf("error = %q, want prefix %q", err.Error(), tc.want)
			}
		})
	}
}
