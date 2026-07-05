package taskrail

import "time"

type Service struct {
	paths Paths
	now   func() time.Time
}

func NewService(start string) (*Service, error) {
	paths, err := DiscoverPaths(start)
	if err != nil {
		return nil, err
	}
	return &Service{paths: paths, now: time.Now}, nil
}
