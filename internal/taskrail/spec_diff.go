package taskrail

import "strings"

// SpecRenameCandidate pairs a removed anchor with an added anchor that share a
// normalized stem. It is a best-effort heuristic surfaced for human review, never
// an assertion that the area was in fact renamed.
type SpecRenameCandidate struct {
	From SpecAnchor `json:"from"`
	To   SpecAnchor `json:"to"`
}

// SpecDiffResult is the read-only anchor-set delta between two versioned specs.
// Added areas are candidates a migration must decompose into tasks; Removed areas
// are ones whose existing tasks become orphaned drift; Renamed pairs are
// best-effort rename candidates only, never asserted as fact.
type SpecDiffResult struct {
	FromVersion string                `json:"from_version"`
	ToVersion   string                `json:"to_version"`
	FromPath    string                `json:"from_path"`
	ToPath      string                `json:"to_path"`
	Added       []SpecAnchor          `json:"added"`
	Removed     []SpecAnchor          `json:"removed"`
	Renamed     []SpecRenameCandidate `json:"renamed"`
}

// SpecDiff computes the mechanical anchor-set delta from <from> to <to>, over
// exactly the anchors spec_ref validation accepts. Version arguments are resolved
// the same way as the rest of the spec family (via SpecShow), so an unknown or
// non-conforming version fails before any diffing work. It is strictly read-only:
// it never writes STATE.md or task files.
func (s *Service) SpecDiff(from, to string) (SpecDiffResult, error) {
	fromSpec, err := s.SpecShow(from, true)
	if err != nil {
		return SpecDiffResult{}, err
	}
	toSpec, err := s.SpecShow(to, true)
	if err != nil {
		return SpecDiffResult{}, err
	}

	added, removed := anchorSetDelta(fromSpec.Anchors, toSpec.Anchors)
	return SpecDiffResult{
		FromVersion: fromSpec.Version,
		ToVersion:   toSpec.Version,
		FromPath:    fromSpec.Path,
		ToPath:      toSpec.Path,
		Added:       added,
		Removed:     removed,
		Renamed:     renameCandidates(added, removed),
	}, nil
}

// anchorSetDelta returns the anchors added in `to` (not in `from`) and removed
// from `from` (not in `to`), each preserving its source document order. The
// returned slices are non-nil so the --json lists marshal as `[]`, not `null`.
func anchorSetDelta(from, to []SpecAnchor) (added, removed []SpecAnchor) {
	added = make([]SpecAnchor, 0, len(to))
	removed = make([]SpecAnchor, 0, len(from))
	fromSet := make(map[string]struct{}, len(from))
	for _, a := range from {
		fromSet[a.Anchor] = struct{}{}
	}
	toSet := make(map[string]struct{}, len(to))
	for _, a := range to {
		toSet[a.Anchor] = struct{}{}
	}
	for _, a := range to {
		if _, ok := fromSet[a.Anchor]; !ok {
			added = append(added, a)
		}
	}
	for _, a := range from {
		if _, ok := toSet[a.Anchor]; !ok {
			removed = append(removed, a)
		}
	}
	return added, removed
}

// renameCandidates greedily matches each removed anchor to the first unused added
// anchor sharing a normalized stem, in document order. The pairing is mechanical
// (shared stem only), supplemental to the definitive delta, and never asserted as
// a real rename.
func renameCandidates(added, removed []SpecAnchor) []SpecRenameCandidate {
	renamed := make([]SpecRenameCandidate, 0, len(removed))
	usedAdded := make([]bool, len(added))

	for _, r := range removed {
		stem := anchorStem(r.Anchor)
		if stem == "" {
			continue
		}
		for i, a := range added {
			if usedAdded[i] || anchorStem(a.Anchor) != stem {
				continue
			}
			usedAdded[i] = true
			renamed = append(renamed, SpecRenameCandidate{From: r, To: a})
			break
		}
	}
	return renamed
}

// anchorStem drops an anchor's final hyphen token, yielding the shared prefix a
// single trailing-word rename preserves (e.g. "spec-coverage-report" and
// "spec-coverage-summary" both stem to "spec-coverage"). Anchors with fewer than
// two tokens have no stem, so they are never treated as rename candidates.
func anchorStem(anchor string) string {
	idx := strings.LastIndex(anchor, "-")
	if idx <= 0 {
		return ""
	}
	return anchor[:idx]
}
