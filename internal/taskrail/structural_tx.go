package taskrail

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/tessariq/taskrail/internal/repotx"
)

// Structural writers change state or specs without owning task fields. They use
// the normal transaction substrate so a projection or spec publication cannot
// be derived from a corpus another writer changed while it was being prepared.
type structuralWriter struct{ command string }

var (
	repairWriter       = structuralWriter{command: "repair"}
	specAddWriter      = structuralWriter{command: "spec add"}
	specActivateWriter = structuralWriter{command: "spec activate"}
)

// testHookReadOnlyRecheck lets tests mutate a consumed path between a read-only
// command's first observation and its final stability check. It is nil in builds.
var testHookReadOnlyRecheck func()

func stableRead[T any](read func() (T, error)) (T, error) {
	first, err := read()
	if err != nil {
		return first, err
	}
	if testHookReadOnlyRecheck != nil {
		testHookReadOnlyRecheck()
	}
	second, err := read()
	if err != nil {
		return second, err
	}
	if !reflect.DeepEqual(first, second) {
		var zero T
		return zero, WithMachineErrorCode(MachineCodeWriteConflict,
			fmt.Errorf("read-only snapshot changed before it could be reported"))
	}
	return second, nil
}

func (s *Service) beginStructuralWriterWrite(w structuralWriter) (repotx.Ownership, func() error, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return nil, nil, err
	}
	if delegatedInvocation() {
		return nil, nil, WithMachineErrorCode(MachineCodeDelegatedRefused,
			fmt.Errorf("delegated loop children cannot invoke %s", w.command))
	}
	return s.acquireWriterLock(w.command, nil)
}

// commitStructuralWriter validates a state/spec candidate and commits only its
// declared files. The full task corpus, its referenced specs, README, config,
// state when it is read rather than written, and command-specific observations
// are all part of the compare-and-swap snapshot.
func (s *Service) commitStructuralWriter(own repotx.Ownership, w structuralWriter, state *State, tasks []*Task, published []repotx.Candidate, extra []repotx.Path) (ValidationResult, error) {
	validation := s.validateInMemory(state, tasks)
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return ValidationResult{}, err
	}
	consumed, err := structuralConsumedPaths(s.paths, tasks, published, extra)
	if err != nil {
		return ValidationResult{}, err
	}
	request := repotx.Request{
		Command:   w.command,
		Consumed:  consumed,
		Published: published,
		Validate: func([]repotx.Snapshot) error {
			if testHookWriterValidated != nil {
				testHookWriterValidated()
			}
			currentTasks, err := s.loadTasks()
			if err != nil {
				return err
			}
			if !sameTaskCorpus(corpus, currentTasks) {
				return fmt.Errorf("%s task corpus changed during candidate validation", w.command)
			}
			// Validate the exact candidate again immediately before the transaction
			// rechecks every consumed and published snapshot.
			_ = s.validateInMemory(state, tasks)
			return nil
		},
	}
	if testHookWriterCandidateBuilt != nil {
		testHookWriterCandidateBuilt()
	}
	if _, err := repotx.Commit(context.Background(), own, request); err != nil {
		return ValidationResult{}, writerTransactionError(err)
	}
	return validation, nil
}

func structuralConsumedPaths(paths Paths, tasks []*Task, published []repotx.Candidate, extra []repotx.Path) ([]repotx.Path, error) {
	consumed, err := writerConsumedPaths(paths, tasks)
	if err != nil {
		return nil, err
	}
	consumed = append(consumed, repotx.Path{
		Kind: repotx.Managed, Reported: paths.logicalManagedPath(paths.StateFile), Physical: paths.StateFile,
	})
	consumed = append(consumed, extra...)
	publishedPhysical := make(map[string]struct{}, len(published))
	for _, candidate := range published {
		publishedPhysical[filepath.Clean(candidate.Physical)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(consumed))
	filtered := make([]repotx.Path, 0, len(consumed))
	for _, path := range consumed {
		if _, ok := publishedPhysical[filepath.Clean(path.Physical)]; ok {
			continue
		}
		key := string(path.Kind) + "\x00" + path.Reported
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, path)
	}
	return filtered, nil
}
