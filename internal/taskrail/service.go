package taskrail

import "time"

type Service struct {
	paths          Paths
	now            func() time.Time
	completionID   func() (string, error)
	verificationID func() (string, error)
	recovery       recoveryAdmission
}

func NewService(start string) (*Service, error) {
	paths, err := DiscoverPaths(start)
	if err != nil {
		// A fenced migration marker with retained transaction state is an
		// interrupted migration: the repository-wide recovery fence names the
		// exact next action, so it outranks the marker's own refusal.
		if MachineFailureFor(err).Code != MachineCodeMigrationInProgress {
			return nil, err
		}
		fenced, fenceErr := DiscoverRecoveryPaths(start)
		if fenceErr != nil {
			return nil, err
		}
		snapshot, snapshotErr := observeRecovery(fenced)
		if snapshotErr != nil || !recoveryRetained(snapshot) {
			return nil, err
		}
		return nil, recoveryPending(fenced, snapshot)
	}
	recovery, err := inspectRecovery(paths)
	if err != nil {
		return nil, err
	}
	return &Service{paths: paths, now: time.Now, completionID: randomCompletionID, verificationID: randomVerificationID, recovery: recovery}, nil
}

// CheckRecovery closes the admission boundary around one semantic operation:
// when it fails, retained transaction state exists now and the caller must
// discard any result. The boundary refuses on the observed state rather than on
// a diff against construction time, because a command that itself publishes and
// clears one durable transaction legitimately changes the tree (including
// leaving an empty transactions directory, which is not retained state).
func (s *Service) CheckRecovery() error {
	current, err := observeRecovery(s.paths)
	if err != nil || recoveryRetained(current) {
		return recoveryPending(s.paths, current)
	}
	return nil
}
