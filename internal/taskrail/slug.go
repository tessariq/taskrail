package taskrail

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// slugNonAlnum matches runs of characters that are not lowercase-alphanumeric, so
// they collapse to a single hyphen after lowercasing.
var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// emptySlugWarnings reports the bare-id fallback for a slug source that was
// supplied but normalized away. The bare id is legitimate, so this is a warning
// rather than an error — but an operator who asked for a slug and silently got
// none has no other signal. An unsupplied source is the intentional bare-id case
// and stays silent. Both `task new` and `task rename` share it so the two paths
// signal identically.
func emptySlugWarnings(source, taskID string) []Warning {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	return []Warning{{
		Code:    "empty_derived_slug",
		Message: fmt.Sprintf("warning: %q produced no slug segment; using bare id %s", source, taskID),
		TaskID:  taskID,
	}}
}

// slugIndivisibleLetters folds the Latin letters that decomposition cannot reach:
// their distinguishing mark is part of the glyph (a stroke, a bar, a ligature)
// rather than a combining character, so NFD leaves them whole and the
// non-alphanumeric collapse would drop the letter entirely. It is a curated list of
// the letters likely to appear in a task title, not an exhaustive enumeration of
// every rune without a canonical decomposition — accented letters never belong
// here, since normalization already handles those.
// Keys are lowercase only, because slugify lowercases before folding.
var slugIndivisibleLetters = strings.NewReplacer(
	"\u00df", "ss", "\u00e6", "ae", "\u0153", "oe", "\u00f8", "o", "\u00f0", "d", "\u00fe", "th",
	"\u0111", "d", "\u0142", "l", "\u0127", "h", "\u014b", "n", "\u0131", "i",
)

// slugUmlauts expands the German umlauts, the one case where dropping the accent
// is not the wanted answer: convention writes u-diaeresis as "ue", not "u". It runs
// on the decomposed form (base letter + U+0308) because slugify normalizes first,
// and it must run before dropCombiningMarks strips that diaeresis.
var slugUmlauts = strings.NewReplacer(
	"a\u0308", "ae", "o\u0308", "oe", "u\u0308", "ue",
)

// dropCombiningMarks removes the nonspacing marks left after the umlaut fold, so a
// decomposed accent leaves its plain base letter behind instead of a hyphen where
// the mark hit the non-alphanumeric collapse.
func dropCombiningMarks(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, s)
}

// slugify normalizes an arbitrary string into a slug: lowercased, accented letters
// folded to ASCII however the input spelled them, non-alphanumeric runs collapsed to
// single hyphens and leading/trailing hyphens trimmed. It underpins the slug segment
// of a task id, so it is deliberately shared between task creation (T-095) and
// rename (T-096) rather than duplicated. A string with no alphanumerics slugifies to
// "" — the caller reads that as "keep the id bare" so a slug segment is never empty.
//
// Decomposing first is what makes the fold general: an accent becomes a separate
// combining mark whatever letter carries it, so dropping marks handles any accented
// letter instead of the Latin-1 ones someone thought to enumerate. It is canonical
// decomposition only — compatibility forms such as the "fi" ligature still fall
// through to the collapse, since NFKD would rewrite far more than accents.
func slugify(s string) string {
	folded := norm.NFD.String(strings.ToLower(strings.TrimSpace(s)))
	folded = slugUmlauts.Replace(folded)
	folded = slugIndivisibleLetters.Replace(folded)
	folded = dropCombiningMarks(folded)
	collapsed := slugNonAlnum.ReplaceAllString(folded, "-")
	return strings.Trim(collapsed, "-")
}
