package taskrail

import (
	"regexp"
	"strings"
)

// slugNonAlnum matches runs of characters that are not lowercase-alphanumeric, so
// they collapse to a single hyphen after lowercasing.
var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify normalizes an arbitrary string into a slug: lowercased, with
// non-alphanumeric runs collapsed to single hyphens and leading/trailing hyphens
// trimmed. It underpins the slug segment of a task id, so it is deliberately shared
// between task creation (T-095) and rename (T-096) rather than duplicated. A string
// with no alphanumerics slugifies to "" — the caller reads that as "keep the id
// bare" so a slug segment is never empty.
func slugify(s string) string {
	lowered := strings.ToLower(strings.TrimSpace(s))
	collapsed := slugNonAlnum.ReplaceAllString(lowered, "-")
	return strings.Trim(collapsed, "-")
}
