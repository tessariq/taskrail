package taskrail

import (
	"strings"
	"testing"
)

// T-402: the spec-review skill must name every member the strict schema-2
// manifest decoder requires of a round lens entry. Both skill-evaluation arms
// first wrote the entry with `filename` and were refused with
// `missing member "path"` because the skill never named the members. The
// documented list is the decoder's own member list, so a decoder change reds
// here until the skill follows it.
func TestSpecReviewSkillDocumentsManifestRoundLensMembers(t *testing.T) {
	assertSkillReferences(t, "taskrail-spec-review",
		"`"+strings.Join(specReview2RoundLensMembers, "`, `")+"`",
	)
}
