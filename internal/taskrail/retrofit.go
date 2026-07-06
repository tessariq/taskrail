package taskrail

import (
	"fmt"
	"strings"
)

// Guided Retrofit Bootstrap Flow (T-035): take an existing repository plus human
// notes and turn them into `specs/`, `planning/tasks/`, and an initial
// `STATE.md`. It composes the pieces already built rather than forking new logic:
// layout detection (T-020) proposes a mapping, structural import (T-033) turns
// the notes into a reviewable planning bootstrap, and the shared dry-run/apply
// primitive (T-018/T-019) scaffolds the layout, writes the marker, and re-runs
// validation. It defaults to a dry run and never overwrites existing files.

// RetrofitInput drives one guided retrofit. NotesPath is an optional human-notes
// markdown file (repo-relative or absolute) imported as a planning bootstrap;
// Apply switches from the default dry run to writing the scaffold.
type RetrofitInput struct {
	NotesPath string
	Apply     bool
}

// RetrofitResult reports what the guided retrofit proposed or did. Mapping is the
// detected layout proposal; Bootstrap is the notes-derived planning draft
// (nil when no notes were given), surfaced for review rather than auto-adopted,
// consistent with import's non-authoritative contract. Changes is the human-
// readable diff of what applying would create; Validation is set only after apply.
type RetrofitResult struct {
	Applied    bool              `json:"applied"`
	Mapping    []RetrofitMapping `json:"mapping,omitempty"`
	Bootstrap  *ImportResult     `json:"bootstrap,omitempty"`
	Changes    []string          `json:"changes,omitempty"`
	Validation *ValidationResult `json:"validation,omitempty"`
}

// Retrofit runs the guided bootstrap flow against an unmarked, non-standard
// repository. It refuses an already-managed repository (one with a marker) so it
// only ever bootstraps, deferring version-aware upgrades to Init. Applying only
// ever creates missing layout content and writes the marker, so human-authored
// files under specs/ and planning/ survive untouched, and the notes file is only
// read.
func (s *Service) Retrofit(input RetrofitInput) (RetrofitResult, error) {
	_, hasMarker, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return RetrofitResult{}, err
	}
	if hasMarker {
		return RetrofitResult{}, fmt.Errorf(
			"repository is already Taskrail-managed (%s exists); use `taskrail init`", markerRelPath())
	}

	bootstrap, err := s.retrofitBootstrap(input.NotesPath)
	if err != nil {
		return RetrofitResult{}, err
	}

	mapping := s.detectRetrofit()
	changes := append(s.pendingLayoutChanges(), markerWriteChange())

	if !input.Apply {
		return RetrofitResult{Mapping: mapping, Bootstrap: bootstrap, Changes: changes}, nil
	}

	validation, err := s.applyLayout(defaultLayoutConfig())
	if err != nil {
		return RetrofitResult{}, err
	}
	return RetrofitResult{
		Applied:    true,
		Mapping:    mapping,
		Bootstrap:  bootstrap,
		Changes:    changes,
		Validation: &validation,
	}, nil
}

// retrofitBootstrap imports the optional notes file into a planning bootstrap
// draft. It reuses the structural import (T-033), so the resulting draft and
// state seed are reviewable proposals a human adopts through the CLI, never
// tracked work the retrofit creates on its own. An empty notes path yields no
// bootstrap.
func (s *Service) retrofitBootstrap(notesPath string) (*ImportResult, error) {
	if strings.TrimSpace(notesPath) == "" {
		return nil, nil
	}
	result, err := s.Import(ImportInput{SourcePath: notesPath, Target: "planning"})
	if err != nil {
		return nil, fmt.Errorf("retrofit bootstrap: %w", err)
	}
	return &result, nil
}
