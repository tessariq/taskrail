package taskrail

import "strings"

// SpecVersionCompletions returns the versioned spec names under specs/ in version
// order, for shell completion of the spec-version positional argument to `spec
// show`/`spec activate`. It reuses SpecList's discovery so completion and listing
// never diverge, and is strictly read-only.
func (s *Service) SpecVersionCompletions() ([]string, error) {
	list, err := s.SpecList()
	if err != nil {
		return nil, err
	}
	versions := make([]string, 0, len(list.Specs))
	for _, spec := range list.Specs {
		versions = append(versions, spec.Version)
	}
	return versions, nil
}

// SpecRefCompletions returns completion candidates for `task new --spec-ref`,
// whose value is a `<path>#<anchor>` pair. Before an anchor is typed it offers
// each spec path with a trailing '#' (the caller keeps the shell from adding a
// space so the anchor phase follows); once a path is fixed it offers that spec's
// spec_ref anchors — exactly the set validation accepts, drawn from the same
// SpecShow --anchors source (T-062), never re-slugged here. It is read-only and
// stays quiet (no candidates, no error) for a path that is not a versioned spec.
func (s *Service) SpecRefCompletions(toComplete string) ([]string, error) {
	list, err := s.SpecList()
	if err != nil {
		return nil, err
	}

	hash := strings.Index(toComplete, "#")
	if hash < 0 {
		paths := make([]string, 0, len(list.Specs))
		for _, spec := range list.Specs {
			paths = append(paths, spec.Path+"#")
		}
		return paths, nil
	}

	path := toComplete[:hash]
	version := ""
	for _, spec := range list.Specs {
		if spec.Path == path {
			version = spec.Version
			break
		}
	}
	if version == "" {
		return nil, nil
	}
	show, err := s.SpecShow(version, true)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0, len(show.Anchors))
	for _, a := range show.Anchors {
		refs = append(refs, path+"#"+a.Anchor)
	}
	return refs, nil
}
