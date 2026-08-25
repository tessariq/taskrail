package taskrail

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
	"gopkg.in/yaml.v3"
)

// The durable layout-2 migration publisher
// (specs/v0.5.0.md#layout-compatibility-and-upgrade). Apply acquires the writer
// lock, rebuilds the exact candidate the preview reported, and publishes it
// through one durable transaction: the fenced marker lands after the originals
// are recorded and before any semantic byte, the candidate write set publishes
// and post-validates, and the strict final marker replaces the fence as the
// transaction's last semantic operation. An interruption leaves the shared
// recovery boundary exactly the retained evidence it needs.

// initMigrationCommand is the canonical command path owning the migration
// transaction, which is also the lock identity and the recovery validator's
// routing key.
const initMigrationCommand = "init"

// applyLayout2Upgrade publishes the gated upgrade candidate under the writer
// lock. The candidate is rebuilt inside the lock and every operator gate is
// re-validated against it, because the authoritative decisions are the ones
// made against the bytes the transaction actually publishes.
func (s *Service) applyLayout2Upgrade(in InitInput) (result InitResult, err error) {
	transactionID, err := newMigrationTransactionID()
	if err != nil {
		return InitResult{}, err
	}
	ctx := context.Background()
	lock, err := repolock.Acquire(ctx, repolock.Request{
		Repository:    s.paths.LockRepository(),
		Command:       initMigrationCommand,
		TransactionID: transactionID,
		Capability:    repolock.Capability{Commands: []string{initMigrationCommand}},
	})
	if err != nil {
		return InitResult{}, migrationLockError(err)
	}
	var committed bool
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil && err == nil {
			// A failed release after a committed migration strands the lock, but
			// the migration itself is complete on disk, exactly like the
			// recovery boundary's own release rule.
			err = WithMachineFailure(
				MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: committed}, releaseErr)
		}
	}()

	candidate, err := buildLayout2MigrationCandidate(s.paths.RepoRoot)
	if err != nil {
		return InitResult{}, err
	}
	if err := validateLayout2UpgradeApplyInputs(in, candidate); err != nil {
		return InitResult{}, err
	}
	request, err := s.migrationTransaction(in, candidate, transactionID)
	if err != nil {
		return InitResult{}, err
	}
	if _, err = durabletx.Run(ctx, lock, s.paths.LockRepository(), request); err != nil {
		return InitResult{}, s.mapMigrationFailure(transactionID, err)
	}
	committed = true

	validation, err := validatePublishedLayout2(s.paths.RepoRoot, candidate)
	if err != nil {
		// The transaction committed; a failed read-back of the published bytes
		// reports an applied failure rather than a refusal that changed nothing.
		return InitResult{}, WithMachineFailure(
			MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: true}, err)
	}
	result = layout2UpgradeResult(candidate)
	result.Outcome = InitMigrated
	result.Applied = true
	result.Validation = validation
	recordAppliedContinuationChoice(&result, in, candidate)
	return result, nil
}

// recordAppliedContinuationChoice fills the note entry the operator's selection
// resolved, so the applied result reports the decision it recorded where the
// preview reported the choices it offered.
func recordAppliedContinuationChoice(result *InitResult, in InitInput, candidate *Layout2MigrationCandidate) {
	if len(candidate.ContinuationNotes) == 0 {
		return
	}
	choice := continuationChoiceDrop
	if in.ExtractContinuationNotes {
		choice = continuationChoiceExtract
	}
	result.Notes[0].ContinuationAction = &choice
}

// migrationTransaction assembles the complete durable request: the fenced
// marker as the fence member, the state and note candidates, every
// refresh-classified skill, and the consumed inputs whose byte stability the
// whole-set comparisons re-prove.
func (s *Service) migrationTransaction(in InitInput, candidate *Layout2MigrationCandidate, transactionID string) (durabletx.Request, error) {
	fenceBytes, err := fencedMarkerBytes(candidate.Marker, transactionID)
	if err != nil {
		return durabletx.Request{}, err
	}
	marker := durabletx.Member{
		Kind: durabletx.Managed, Reported: candidate.MarkerPath, Path: candidate.MarkerPath,
		Content: candidate.MarkerBytes, Fence: fenceBytes,
	}
	members := []durabletx.Member{marker, {
		Kind: durabletx.Managed, Reported: candidate.StatePath, Path: candidate.StatePath,
		Content: candidate.StateBytes,
	}}
	expected := map[string]string{
		candidate.MarkerPath: digestBytes(candidate.MarkerBytes),
		candidate.StatePath:  digestBytes(candidate.StateBytes),
	}
	notesMember := durabletx.Path{Kind: durabletx.Managed, Reported: candidate.NotesPath, Path: candidate.NotesPath}
	notesExpectedAbsent := false
	if !candidate.NotesPresent {
		notesExpectedAbsent = true
		notes := []byte(starterNotes())
		if in.ExtractContinuationNotes {
			notes = candidate.NotesExtractionBytes
		}
		members = append(members, durabletx.Member{
			Kind: durabletx.Managed, Reported: candidate.NotesPath, Path: candidate.NotesPath, Content: notes,
		})
		expected[candidate.NotesPath] = digestBytes(notes)
	}
	skillMembers, err := migrationSkillMembers(in, candidate)
	if err != nil {
		return durabletx.Request{}, err
	}
	for _, skill := range skillMembers {
		members = append(members, skill)
		expected[skill.Reported] = digestBytes(skill.Content)
	}
	var consumed []durabletx.Path
	for logical := range candidate.TaskBytes {
		consumed = append(consumed, durabletx.Path{Kind: durabletx.Managed, Reported: logical, Path: logical})
	}
	for _, skill := range candidate.Skills {
		if skill.Outcome != migrationSkillParity {
			continue
		}
		// A committed parity mirror is byte-compared again at publication, so
		// the marker never publishes over a mirror that diverged since preview.
		consumed = append(consumed, durabletx.Path{Kind: durabletx.Worktree, Reported: skill.Path, Path: skill.Path})
	}
	if candidate.NotesPresent {
		consumed = append(consumed, notesMember)
	}
	return durabletx.Request{
		Command:  initMigrationCommand,
		Members:  members,
		Consumed: consumed,
		Validate: migrationValidator(expected, digestBytes(fenceBytes), candidate.NotesPath, notesExpectedAbsent),
	}, nil
}

// migrationValidator is the transaction's own command check: the recorded
// candidate digests are exactly the ones this migration decided, the fence
// digest is the fenced marker it named, and a note destination classified
// absent is still recorded absent at preparation.
func migrationValidator(expected map[string]string, fenceDigest, notesPath string, notesExpectedAbsent bool) func([]durabletx.Evidence) error {
	return func(snapshots []durabletx.Evidence) error {
		for _, snapshot := range snapshots {
			want, ok := expected[snapshot.Reported]
			if !ok {
				continue
			}
			if snapshot.CandidateSHA256 != want {
				return fmt.Errorf("candidate for %s does not match the migration decision", snapshot.Reported)
			}
			if snapshot.Reported == markerRelPath() {
				if snapshot.FenceSHA256 != fenceDigest {
					return fmt.Errorf("fence bytes for %s do not match the migration decision", snapshot.Reported)
				}
			}
			if notesExpectedAbsent && snapshot.Reported == notesPath && snapshot.OriginalSHA256 != "" {
				return WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf(
					"notes sidecar %s appeared since the preview; merge it manually or remove it, then re-run the apply", notesPath))
			}
		}
		return nil
	}
}

// migrationSkillMembers stamps the embedded package bytes for every
// refresh-classified destination, normalizing legacy and dual markers to the
// nested-only form.
func migrationSkillMembers(in InitInput, candidate *Layout2MigrationCandidate) ([]durabletx.Member, error) {
	var members []durabletx.Member
	for _, skill := range candidate.Skills {
		if skill.Outcome != migrationSkillRefresh {
			continue
		}
		packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, skill.PackageRel))
		if err != nil {
			return nil, err
		}
		stamped, err := stampSkillVersion(packaged, in.SkillVersion)
		if err != nil {
			return nil, fmt.Errorf("stamp migration skill %s: %w", skill.Path, err)
		}
		if err := validateAgentSkill(stamped); err != nil {
			return nil, fmt.Errorf("validate migration skill %s: %w", skill.Path, err)
		}
		members = append(members, durabletx.Member{
			Kind: durabletx.Worktree, Reported: skill.Path, Path: skill.Path, Content: stamped,
		})
	}
	return members, nil
}

// fencedMarkerBytes renders the exact temporary shape the spec fixes for the
// in-progress fence and proves it decodes in that exact shape before the
// transaction can publish it.
func fencedMarkerBytes(marker Layout2Config, transactionID string) ([]byte, error) {
	fenced := marker
	fenced.MigrationFence = &Layout2MigrationFence{FromLayoutVersion: currentLayoutVersion, TransactionID: transactionID}
	data, err := yaml.Marshal(fenced)
	if err != nil {
		return nil, fmt.Errorf("marshal fenced marker: %w", err)
	}
	decoded, err := decodeLayoutMarkerStrict(data)
	if err != nil || decoded.MigrationFence == nil || decoded.MigrationFence.TransactionID != transactionID {
		return nil, fmt.Errorf("validate fenced marker candidate: %v", err)
	}
	return data, nil
}

// validatePublishedLayout2 re-reads the published repository with the strict
// decoders, so the applied result's validation attests the bytes that landed
// rather than the bytes the candidate intended.
func validatePublishedLayout2(root string, candidate *Layout2MigrationCandidate) (*ValidationResult, error) {
	violations := strictLayout2Violations(root, candidate.Marker.PlanningDir)
	if len(violations) > 0 {
		return nil, fmt.Errorf("validate published layout 2: %s", strings.Join(violations, "; "))
	}
	markerData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(candidate.MarkerPath)))
	if err != nil {
		return nil, fmt.Errorf("read published marker: %w", fsCause(err))
	}
	if !bytes.Equal(markerData, candidate.MarkerBytes) {
		return nil, fmt.Errorf("published marker is not the candidate the preview reported")
	}
	stateData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(candidate.Marker.PlanningDir), "STATE.md"))
	if err != nil {
		return nil, fmt.Errorf("read published state: %w", fsCause(err))
	}
	if !bytes.Equal(stateData, candidate.StateBytes) {
		return nil, fmt.Errorf("published state is not the candidate the preview reported")
	}
	for logical, want := range candidate.TaskBytes {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(logical)))
		if err != nil || !bytes.Equal(data, want) {
			return nil, fmt.Errorf("published task %s is not the byte-preserved candidate: %w", logical, fsCause(err))
		}
	}
	return &ValidationResult{Valid: true, Violations: []string{}}, nil
}

// strictLayout2Violations decodes one repository's marker, state, and task set
// with the strict layout-2 readers and reports every violation. The recovery
// boundary uses it directly for a completed migration, because this binary's
// schema-1 Validate cannot read state schema 2 as valid yet.
func strictLayout2Violations(root, planningDir string) []string {
	var violations []string
	markerData, err := os.ReadFile(filepath.Join(root, taskrailConfigDir, taskrailConfigFile))
	if err != nil {
		return append(violations, fmt.Sprintf("read published marker: %v", fsCause(err)))
	}
	marker, err := decodeLayoutMarkerStrict(markerData)
	if err != nil {
		return append(violations, fmt.Sprintf("validate published marker: %v", err))
	}
	if marker.MigrationFence != nil {
		violations = append(violations, "published marker still carries its migration fence")
	}
	planning := filepath.Join(root, filepath.FromSlash(planningDir))
	stateData, err := os.ReadFile(filepath.Join(planning, "STATE.md"))
	if err != nil {
		return append(violations, fmt.Sprintf("read published state: %v", fsCause(err)))
	}
	state, _, err := decodeStateStrict(stateData)
	if err != nil {
		return append(violations, fmt.Sprintf("validate published state: %v", err))
	}
	entries, err := os.ReadDir(filepath.Join(planning, "tasks"))
	if err != nil {
		return append(violations, fmt.Sprintf("read published tasks: %v", fsCause(err)))
	}
	var tasks []*Task
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(planning, "tasks", entry.Name()))
		if err != nil {
			violations = append(violations, fmt.Sprintf("read published task %s: %v", entry.Name(), fsCause(err)))
			continue
		}
		task, err := decodeMigrationTaskStrict(data)
		if err != nil {
			violations = append(violations, fmt.Sprintf("validate published task %s: %v", entry.Name(), err))
			continue
		}
		tasks = append(tasks, task)
	}
	if err := validateMigrationStateTaskLinks(state.Frontmatter, tasks); err != nil {
		violations = append(violations, err.Error())
	}
	return violations
}

// validateInitRecovery is the init migration's registered recovery validator:
// accept-candidate is permitted only when the retained evidence still names the
// fenced marker, and the retained final marker bytes decode as the strict final
// layout-2 shape. It reads only recorded transaction evidence, never
// re-derived repository content.
func (s *Service) validateInitRecovery(transactionID string, snapshots []durabletx.Evidence) error {
	markerReported := markerRelPath()
	for _, snapshot := range snapshots {
		if snapshot.Kind != durabletx.Worktree || snapshot.Reported != markerReported || snapshot.CandidateSHA256 == "" {
			continue
		}
		data, err := os.ReadFile(s.paths.ConfigFile)
		if err != nil || digestBytes(data) != snapshot.CandidateSHA256 {
			return fmt.Errorf("local init marker no longer matches retained candidate")
		}
		marker, err := decodeLayoutMarkerStrict(data)
		if err != nil || marker.StorageMode != StorageLocal || marker.MigrationFence != nil {
			return fmt.Errorf("retained local init marker is invalid")
		}
		if err := s.validateRecoveredLocalSkills(snapshots); err != nil {
			return err
		}
		return nil
	}
	var found bool
	for _, snapshot := range snapshots {
		if snapshot.Kind != durabletx.Managed || snapshot.Reported != markerReported {
			continue
		}
		found = true
		if snapshot.FenceSHA256 == "" {
			return fmt.Errorf("migration transaction does not fence %s", markerReported)
		}
	}
	if !found {
		return fmt.Errorf("migration transaction does not publish %s", markerReported)
	}
	final, err := durabletx.RetainedCandidate(s.paths.LockRepository(), transactionID, durabletx.Managed, markerReported)
	if err != nil {
		return fmt.Errorf("read retained migration marker: %w", err)
	}
	marker, err := decodeLayoutMarkerStrict(final)
	if err != nil {
		return fmt.Errorf("retained migration marker is not a strict layout 2 marker: %w", err)
	}
	if marker.MigrationFence != nil {
		return fmt.Errorf("retained migration marker still carries a fence")
	}
	return nil
}

func (s *Service) validateRecoveredLocalSkills(snapshots []durabletx.Evidence) error {
	installed := false
	for _, snapshot := range snapshots {
		if snapshot.Kind == durabletx.Worktree && isSkillDestination(snapshot.Reported) {
			installed = true
			break
		}
	}
	if !installed {
		return nil
	}
	plan, err := s.planLocalSkills()
	if err != nil {
		return err
	}
	if err := validateFreshLocalSkillPlan(plan, localSkillRefresh, localSkillExclusionManaged); err != nil {
		return err
	}
	return s.verifyLocalIgnored()
}

// mapMigrationFailure classifies a durable migration outcome onto the command's
// registered error subset, carrying the typed whole-set snapshots and the
// recovery reference a refusal leaves behind. A fully rolled-back transaction
// keeps its own cause's classification: the repository is back to its
// originals, so what remains to report is why it rolled back.
func (s *Service) mapMigrationFailure(transactionID string, err error) error {
	var txErr *durabletx.Error
	if !errors.As(err, &txErr) {
		return err
	}
	code, ok := txErr.MachineCode()
	if !ok {
		if txErr.Kind == durabletx.KindRolledBack {
			failure := MachineFailureFor(txErr.Unwrap())
			failure.Snapshots = recoverySnapshots(txErr.Snapshots())
			return WithMachineFailure(failure, err)
		}
		code = MachineCodeRepositoryInvalid
	}
	failure := MachineFailure{
		Code:      code,
		Snapshots: recoverySnapshots(txErr.Snapshots()),
		Recovery:  s.recoveryRef(transactionID),
	}
	return WithMachineFailure(failure, err)
}

// migrationLockError maps the apply's lock acquisition refusals: the writer lock
// is the migration's first requirement, and its holders are inspected and
// cleared through the guarded lock surface.
func migrationLockError(err error) error {
	switch {
	case errors.Is(err, repolock.ErrHeld), errors.Is(err, repolock.ErrSameProcess):
		return WithMachineErrorCode(MachineCodeLockHeld, fmt.Errorf(
			"the repository mutation lock is held: inspect it with taskrail lock status and clear an abandoned owner with taskrail lock clear before migrating: %w", err))
	case errors.Is(err, repolock.ErrMalformed):
		return WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("repository mutation lock metadata is unreadable: %w", err))
	default:
		return err
	}
}

func newMigrationTransactionID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate migration transaction id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
