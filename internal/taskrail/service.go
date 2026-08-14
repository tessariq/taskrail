package taskrail

import "time"

type Service struct {
	paths    Paths
	now      func() time.Time
	recovery recoveryAdmission
}

func NewService(start string) (*Service, error) {
	paths, err := DiscoverPaths(start)
	if err != nil {
		return nil, err
	}
	recovery, err := inspectRecovery(paths)
	if err != nil {
		return nil, err
	}
	return &Service{paths: paths, now: time.Now, recovery: recovery}, nil
}

// CheckRecovery closes the admission boundary around one semantic operation.
// Callers must discard any result when it fails.
func (s *Service) CheckRecovery() error {
	current, err := observeRecovery(s.paths)
	if err != nil || !s.recovery.snapshot.Same(current) || recoveryRetained(current) {
		return recoveryPending(s.paths, current)
	}
	return nil
}
