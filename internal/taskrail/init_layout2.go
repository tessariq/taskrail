package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

// The layout-2 upgrade surface of init (specs/v0.5.0.md#layout-compatibility-
// and-upgrade). A repository whose marker sits at this binary's current layout
// while a newer layout is modeled is upgradable: its flagless init becomes a
// complete, write-free preview of every operator decision — candidate paths,
// committed storage, the default broad review-round maximum, decoded
// continuation notes with their extract/drop choices, per-skill classifications,
// and the candidate's validation outcome — and its `--apply` validates every
// operator gate and then publishes the exact previewed candidate through the
// durable migration fence (init_layout2_apply.go).

// digestBytes is the digest over the exact candidate bytes a future apply
// would publish, so preview and apply name one candidate, not two computations.
func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// upgradableCurrentLayout reports whether a found marker pins exactly the
// binary's current layout while a newer layout is modeled, which is the one
// classification init can preview an upgrade for. When a later slice raises the
// current layout to the modeled maximum, this predicate goes false and the
// older-marker migration branch in planInit receives layout-1 repositories;
// that slice must re-route this upgrade flow onto the durable publisher rather
// than inheriting planInit's non-strict scaffold migration.
func upgradableCurrentLayout(cfg LayoutConfig) bool {
	return cfg.LayoutVersion == currentLayoutVersion && currentLayoutVersion < layout2Version
}

// initLayout2Upgrade serves the layout-2 upgrade preview and its gated apply.
// Both build the identical strict candidate from the same repository bytes, so
// the decisions an operator reads from the preview are exactly the ones the
// apply gates enforce.
func (s *Service) initLayout2Upgrade(in InitInput) (InitResult, error) {
	candidate, err := buildLayout2MigrationCandidate(s.paths.RepoRoot)
	if err != nil {
		// Refusals that already carry a code (path_blocked, destination_exists,
		// unsupported legacy input) keep it; strict-decode failures surface as
		// repository_invalid, the one conclusion that holds for bytes this
		// command could not read as a migratable layout.
		return InitResult{}, err
	}
	if !in.Apply {
		if err := rejectUpgradeApplyInputs(in); err != nil {
			return InitResult{}, err
		}
		return s.reportLayout2UpgradePreview(candidate), nil
	}
	if err := validateLayout2UpgradeApplyInputs(in, candidate); err != nil {
		return InitResult{}, err
	}
	return s.applyLayout2Upgrade(in)
}

// reportLayout2UpgradePreview renders the candidate as the migration-preview
// result. Validation is the candidate's own strict outcome: construction already
// round-trips the marker and state through the strict decoders, so a reported
// preview asserts the migration would publish decodable bytes.
func (s *Service) reportLayout2UpgradePreview(candidate *Layout2MigrationCandidate) InitResult {
	result := layout2UpgradeResult(candidate)
	result.Outcome = InitMigrationPreview
	return result
}

// layout2UpgradeResult is the shared projection of one layout-2 migration
// candidate: the exact paths, decisions, and facts preview and apply both
// report, so an operator comparing the two reads one set of facts.
func layout2UpgradeResult(candidate *Layout2MigrationCandidate) InitResult {
	notes := candidate.ContinuationNotes
	if notes == nil {
		// The contract's continuation_notes array is required and non-null.
		notes = []string{}
	}
	fileAction := noteActionCreateTemplate
	if candidate.NotesPresent {
		fileAction = noteActionPreserve
	}
	choices := []string{}
	if len(notes) > 0 {
		if !candidate.NotesPresent {
			choices = append(choices, continuationChoiceExtract)
		}
		choices = append(choices, continuationChoiceDrop)
	}
	digest := digestBytes(candidate.MarkerBytes)
	return InitResult{
		FromVersion: currentLayoutVersion,
		ToVersion:   layout2Version,
		Applied:     false,
		StorageMode: string(candidate.Marker.StorageMode),
		Config: InitConfig{
			Path:            candidate.MarkerPath,
			Action:          configActionMigrate,
			CandidateSHA256: digest,
		},
		Writes:            layout2UpgradeWrites(candidate),
		Notes:             []InitNote{{Path: candidate.NotesPath, FileAction: fileAction, ContinuationAction: nil, ContinuationChoices: choices}},
		Skills:            layout2UpgradeSkills(candidate),
		SkillExclusions:   []InitSkillExclusion{},
		ContinuationNotes: notes,
		Validation:        &ValidationResult{Valid: true, Violations: []string{}},
		Layout2Facts: &Layout2UpgradeFacts{
			ReviewMaxRounds: candidate.Marker.ImplementationReviewMaxRounds,
			StorageRoot:     committedStorageRoot,
			SpecsDir:        candidate.Marker.SpecsDir,
			PlanningDir:     candidate.Marker.PlanningDir,
		},
	}
}

// layout2UpgradeWrites inventories the candidate's complete path set in path
// order: the rewritten marker, the schema-2 state, the notes destination the
// outcome creates or preserves, and every task file the migration preserves
// byte-for-byte. Skills stay out of this list: they are reported through their
// own classification inventory.
func layout2UpgradeWrites(candidate *Layout2MigrationCandidate) []WriteEntry {
	noteAction := writeActionCreate
	if candidate.NotesPresent {
		noteAction = writeActionPreserve
	}
	writes := []WriteEntry{
		{Path: candidate.MarkerPath, Kind: writeKindConfig, Action: writeActionRefresh},
		{Path: candidate.NotesPath, Kind: writeKindNote, Action: noteAction},
		{Path: candidate.StatePath, Kind: writeKindState, Action: writeActionRefresh},
	}
	for logical := range candidate.TaskBytes {
		writes = append(writes, WriteEntry{Path: logical, Kind: writeKindTask, Action: writeActionPreserve})
	}
	slices.SortFunc(writes, func(a, b WriteEntry) int { return strings.Compare(a.Path, b.Path) })
	return writes
}

// layout2UpgradeSkills maps the candidate's classifications onto the skill
// inventory vocabulary: a committed parity mirror is preserved, and every
// stamped, legacy-only, or matching-dual copy is refreshed by the combined
// forced install. Blocking copies never reach here — classification refused
// them before the preview could report a decision.
func layout2UpgradeSkills(candidate *Layout2MigrationCandidate) []InitSkill {
	skills := make([]InitSkill, 0, len(candidate.Skills))
	for _, skill := range candidate.Skills {
		action := writeActionRefresh
		if skill.Outcome == migrationSkillParity {
			action = writeActionPreserve
		}
		skills = append(skills, InitSkill{Path: skill.Path, Action: action})
	}
	return skills
}

// rejectUpgradeApplyInputs keeps the preview honest: the quiescence assertion
// and the note selections are apply inputs, and a preview that accepted them
// would imply a recorded decision its own report leaves null.
func rejectUpgradeApplyInputs(in InitInput) error {
	if in.ConfirmQuiescent {
		return invalidArgumentsf("--confirm-quiescent asserts quiescence for the layout upgrade apply; it is invalid for a preview")
	}
	if in.ExtractContinuationNotes || in.DropContinuationNotes {
		return invalidArgumentsf("the continuation-note selection is recorded by the layout upgrade apply; re-run with --apply to select it")
	}
	return nil
}

// validateLayout2UpgradeApplyInputs enforces every operator gate the preview
// reported: explicit quiescence, exactly the note selection the decoded notes
// make applicable, and the combined forced skill install whenever a stamped
// copy requires normalization.
func validateLayout2UpgradeApplyInputs(in InitInput, candidate *Layout2MigrationCandidate) error {
	if !in.ConfirmQuiescent {
		return invalidArgumentsf("layout upgrade apply requires --confirm-quiescent: confirm every older Taskrail process able to touch this repository or its linked-worktree storage has stopped")
	}
	if err := validateUpgradeNoteSelection(in, candidate); err != nil {
		return err
	}
	for _, skill := range candidate.Skills {
		if skill.Outcome != migrationSkillRefresh {
			continue
		}
		if !in.WithSkills || !in.ForceSkills {
			return invalidArgumentsf("installed skill %s requires the combined forced refresh: taskrail init --apply --with-skills --force", skill.Path)
		}
		break
	}
	return nil
}

func validateUpgradeNoteSelection(in InitInput, candidate *Layout2MigrationCandidate) error {
	notes := candidate.ContinuationNotes
	if in.ExtractContinuationNotes && in.DropContinuationNotes {
		return invalidArgumentsf("choose exactly one of --extract-continuation-notes or --drop-continuation-notes")
	}
	if len(notes) == 0 {
		option := "--extract-continuation-notes or --drop-continuation-notes"
		if in.ExtractContinuationNotes || in.DropContinuationNotes {
			if candidate.SourceStateSchema >= 2 {
				return invalidArgumentsf("source state is already schema 2, so %s does not apply", option)
			}
			return invalidArgumentsf("no continuation notes to preserve: %s is unnecessary", option)
		}
		return nil
	}
	if !in.ExtractContinuationNotes && !in.DropContinuationNotes {
		return invalidArgumentsf("decoded continuation notes require exactly one of --extract-continuation-notes or --drop-continuation-notes")
	}
	if in.ExtractContinuationNotes && candidate.NotesPresent {
		// The same refusal the extraction candidate itself produces: the merge
		// of human-owned prose is the operator's, never Taskrail's.
		return WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf(
			"notes sidecar %s already exists; merge the continuation notes manually, then re-run with --drop-continuation-notes",
			candidate.NotesPath))
	}
	return nil
}

// rejectUpgradeOnlyInputs refuses the three layout-upgrade inputs everywhere the
// upgrade flow does not serve them, so an operator never has a flag silently
// accepted by an outcome that ignores it.
func rejectUpgradeOnlyInputs(in InitInput) error {
	if in.ConfirmQuiescent {
		return invalidArgumentsf("--confirm-quiescent asserts quiescence for the layout %d upgrade apply and is invalid here", layout2Version)
	}
	if in.ExtractContinuationNotes || in.DropContinuationNotes {
		return invalidArgumentsf("--extract-continuation-notes/--drop-continuation-notes record the layout %d note decision and are invalid here", layout2Version)
	}
	return nil
}
