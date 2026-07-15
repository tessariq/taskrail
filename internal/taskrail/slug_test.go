package taskrail

import "testing"

func TestSlugify(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "AddSlug", "addslug"},
		{"collapses non-alphanumeric runs", "Add slug support!", "add-slug-support"},
		{"trims leading and trailing hyphens", "  --Add--  ", "add"},
		{"collapses mixed punctuation", "Cross-league OVR (comparability)", "cross-league-ovr-comparability"},
		{"keeps digits", "v0.4.0 release", "v0-4-0-release"},
		{"empty when no alphanumerics", "!!! --- ???", ""},
		{"already a slug is idempotent", "league-strength-coefficients", "league-strength-coefficients"},
		{"empty input", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := slugify(tc.in); got != tc.want {
				t.Fatalf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
