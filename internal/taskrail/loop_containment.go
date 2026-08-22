package taskrail

import "os/exec"

// loopChildContainmentEvidence records only processes the platform can observe.
// A child that deliberately escapes its process group is outside this boundary.
type loopChildContainmentEvidence struct {
	Platform              string
	ProcessGroup          int
	NormalDrain           bool
	TerminationRequested  bool
	ForcedTermination     bool
	Survivors             bool
	SurvivorPIDs          []int
	InspectionError       string
	ObservationLimitation string
	leaderReaped          bool
	leaderExitCode        int
	leaderSignal          string
}

type loopChildContainment interface {
	configure(*exec.Cmd)
	verify(pid int) (loopChildContainmentEvidence, error)
	exited(pid int) (bool, int, string, error)
	signal(error) string
	cleanup() (loopChildContainmentEvidence, error)
}
