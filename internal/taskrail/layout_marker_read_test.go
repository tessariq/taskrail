package taskrail

import (
	"os"
	"strings"
	"testing"
)

// The two marker readers share a read/unmarshal/version-guard sequence but
// deliberately report it with different adopter-facing wording: discovery calls
// the file a "layout config", init calls it a "layout marker". A fold of the
// shared sequence must not collapse that wording (T-131).
func TestLayoutMarkerReadErrorsKeepCallerWording(t *testing.T) {
	t.Parallel()

	seedUnparsable := func(t *testing.T, repo string) {
		t.Helper()
		writeFile(t, markerFile(repo), "layout_version: [not-an-int\n")
	}
	// A directory where the marker file belongs makes os.ReadFile fail with
	// something other than ErrNotExist, which is the only way to reach the
	// read-error branch rather than the absent-marker branch.
	seedUnreadable := func(t *testing.T, repo string) {
		t.Helper()
		if err := os.MkdirAll(markerFile(repo), 0o755); err != nil {
			t.Fatalf("mkdir marker path: %v", err)
		}
	}

	cases := []struct {
		name string
		read func(string) error
		seed func(*testing.T, string)
		want string
	}{
		{
			name: "loadLayoutConfig/parse",
			read: func(repo string) error { _, err := loadLayoutConfig(repo); return err },
			seed: seedUnparsable,
			want: "parse layout config",
		},
		{
			name: "loadLayoutConfig/read",
			read: func(repo string) error { _, err := loadLayoutConfig(repo); return err },
			seed: seedUnreadable,
			want: "read layout config",
		},
		{
			name: "readMarker/parse",
			read: func(repo string) error { _, _, err := readMarker(repo); return err },
			seed: seedUnparsable,
			want: "parse layout marker",
		},
		{
			name: "readMarker/read",
			read: func(repo string) error { _, _, err := readMarker(repo); return err },
			seed: seedUnreadable,
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
