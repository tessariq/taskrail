package taskrail

import (
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

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
		{"folds german umlauts and eszett", "Über Fußball", "ueber-fussball"},
		{"folds uppercase umlauts", "ÄRGER Öl", "aerger-oel"},
		{"folds accented latin to base letters", "Café Niño Français", "cafe-nino-francais"},
		{"folds latin-1 ligatures and extras", "Æther Œuvre Ångström Þing", "aether-oeuvre-angstroem-thing"},
		{"empty when only non-latin script", "日本語", ""},
		// NFD input: a base letter plus a combining mark, which macOS terminals and
		// some IMEs emit instead of the precomposed rune. Spelled with explicit
		// escapes so an editor cannot silently re-normalize the fixture away.
		{"folds decomposed umlauts to the two-letter form", "U\u0308ber Fußball", "ueber-fussball"},
		{"folds decomposed accents to the base letter", "Cafe\u0301 Nin\u0303o Franc\u0327ais", "cafe-nino-francais"},
		{"drops a stray combining mark", "a\u0301\u0301", "a"},
		{"empty when only combining marks", "\u0308\u0301", ""},
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

// TestSlugifyFoldsBothNormalizationForms pins what real normalization buys over a
// hand-listed table: how an operator's keyboard encoded an accent must not change
// the id a task gets, and that has to hold for any accented letter rather than the
// Latin-1 set someone thought to enumerate. The last four rows are letters no
// hand-written list covered.
//
// The decomposed column is spelled with explicit escapes, and each column is
// checked to actually be in the form its name claims. Without that check the two
// spellings are indistinguishable on screen, so a raw literal an editor
// re-normalized would leave a row asserting one spelling twice and still passing.
func TestSlugifyFoldsBothNormalizationForms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		precomposed string
		decomposed  string
		want        string
	}{
		{"german umlaut and eszett", "Über Fußball", "U\u0308ber Fußball", "ueber-fussball"},
		{"french and spanish accents", "Café Niño Français", "Cafe\u0301 Nin\u0303o Franc\u0327ais", "cafe-nino-francais"},
		{"uppercase umlauts", "Ärger Öl", "A\u0308rger O\u0308l", "aerger-oel"},
		{"ring above and umlaut", "Ångström", "A\u030angstro\u0308m", "angstroem"},
		{"w with diaeresis", "ẅ", "w\u0308", "w"},
		{"vietnamese e circumflex acute", "ế", "e\u0302\u0301", "e"},
		{"s with caron", "š", "s\u030c", "s"},
		{"a with macron", "ā", "a\u0304", "a"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := norm.NFC.String(tc.precomposed); got != tc.precomposed {
				t.Fatalf("fixture %q: precomposed column is not NFC; it must be spelled %q", tc.name, got)
			}
			if got := norm.NFD.String(tc.decomposed); got != tc.decomposed {
				t.Fatalf("fixture %q: decomposed column is not NFD; it must be spelled %q", tc.name, got)
			}
			if got := slugify(tc.precomposed); got != tc.want {
				t.Fatalf("slugify(precomposed %q) = %q, want %q", tc.precomposed, got, tc.want)
			}
			if got := slugify(tc.decomposed); got != tc.want {
				t.Fatalf("slugify(NFD %q) = %q, want %q", tc.decomposed, got, tc.want)
			}
		})
	}
}

// TestSlugifyStackedMarksCharacterizeOrdering pins where normalization stops
// helping. NFD's canonical ordering does not reorder marks of equal combining
// class, so whether an umlaut expands depends on the order the marks arrive in: a
// real precomposed character always decomposes with the diaeresis adjacent to its
// base (so it expands), while hand-stacked input that puts another accent first
// leaves the diaeresis unattached and the letter folds to its base. Both are
// defensible — u-with-acute-and-diaeresis is not a German umlaut — but neither
// should change silently.
func TestSlugifyStackedMarksCharacterizeOrdering(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"precomposed u with diaeresis and acute decomposes diaeresis-first", "\u01d8", "ue"},
		{"diaeresis adjacent to the base expands", "u\u0308\u0301", "ue"},
		{"another accent between base and diaeresis folds to the base", "u\u0301\u0308", "u"},
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

// TestSlugifyFoldsStrokeAndBarLetters covers the other class of letter that
// normalization cannot reach: a stroke or bar is part of the glyph, not a combining
// mark, so these have no canonical decomposition and would otherwise vanish into
// the non-alphanumeric collapse exactly like the accented letters T-118 rescued.
func TestSlugifyFoldsStrokeAndBarLetters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"d with stroke", "\u0111", "d"},
		{"l with stroke", "\u0142", "l"},
		{"h with stroke", "\u0127", "h"},
		{"eng", "\u014b", "n"},
		{"dotless i", "\u0131", "i"},
		{"crossed o", "\u00f8", "o"},
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

// TestSlugifyLeavesCompatibilityLigatures characterizes a deliberate limit of the
// canonical fold. NFD does not expand compatibility ligatures — only NFKD does —
// so a pasted "\ufb01" ligature drops out rather than becoming "fi". NFKD is not used
// because it also rewrites unrelated compatibility forms (superscripts, enclosed
// and unit characters), which is a much larger behavior change than a slug needs.
// Pinning the current answer keeps that trade visible instead of implicit.
func TestSlugifyLeavesCompatibilityLigatures(t *testing.T) {
	t.Parallel()

	if got := slugify("\ufb01nal"); got != "nal" {
		t.Fatalf("slugify(fi-ligature + \"nal\") = %q, want %q", got, "nal")
	}
	// The plain two-letter spelling is unaffected, so this only bites pasted text.
	if got := slugify("final"); got != "final" {
		t.Fatalf("slugify(%q) = %q, want %q", "final", got, "final")
	}
}

// TestCapSlug pins the title-derived length cap: a long slug is bounded near
// slugMaxLen, trimmed on a hyphen boundary so no token is cut mid-word and no
// leading/trailing hyphen survives, while a slug already within budget is
// returned untouched.
func TestCapSlug(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"short slug untouched", "add-slug-support", "add-slug-support"},
		{
			name: "long slug trimmed on hyphen boundary",
			in:   "cap-the-title-derived-slug-length-at-roughly-fifty-characters-boundary-aware",
			want: "cap-the-title-derived-slug-length-at-roughly-fifty",
		},
		{
			name: "single mega-token falls back rather than splitting the token",
			in:   "supercalifragilisticexpialidocioussupercalifragilisticexpialidocious",
			want: "",
		},
		{
			name: "token ending exactly at cap is retained",
			in:   strings.Repeat("a", slugMaxLen) + "-next",
			want: strings.Repeat("a", slugMaxLen),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := capSlug(tc.in)
			if got != tc.want {
				t.Fatalf("capSlug(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if len(got) > slugMaxLen {
				t.Fatalf("capSlug(%q) len %d exceeds cap %d", tc.in, len(got), slugMaxLen)
			}
			if got != strings.Trim(got, "-") {
				t.Fatalf("capSlug(%q) = %q has a stray boundary hyphen", tc.in, got)
			}
		})
	}
}
