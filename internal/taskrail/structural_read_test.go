package taskrail

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestStructuralReadOnlyCommandsRefuseChangedSnapshotsWithoutLocking(t *testing.T) {
	for _, command := range []string{"repair", "spec list", "spec show", "spec diff"} {
		t.Run(command, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Summary\n")
			svc := newTestService(t, repo, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
			testHookReadOnlyRecheck = func() {
				switch command {
				case "repair":
					writeFile(t, svc.paths.StateFile, readBytes(t, svc.paths.StateFile)+"<!-- changed -->\n")
				case "spec list":
					writeFile(t, filepath.Join(repo, "specs", "v0.3.0.md"), "# Taskrail v0.3.0\n")
				case "spec show":
					writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# changed\n")
				case "spec diff":
					writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# changed\n")
				}
			}
			t.Cleanup(func() { testHookReadOnlyRecheck = nil })

			var err error
			switch command {
			case "repair":
				_, err = svc.Repair(RepairInput{})
			case "spec list":
				_, err = svc.SpecList()
			case "spec show":
				_, err = svc.SpecShow("v0.2.0", false)
			case "spec diff":
				_, err = svc.SpecDiff("v0.1.0", "v0.2.0")
			}
			if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
				t.Fatalf("%s after an external edit = %v, want write_conflict", command, err)
			}
			status, statusErr := repolock.Inspect(svc.paths.LockRepository())
			if statusErr != nil || status.Held {
				t.Fatalf("%s created or held a mutation lock: %+v, %v", command, status, statusErr)
			}
		})
	}
}
