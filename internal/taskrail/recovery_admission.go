package taskrail

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
)

const recoveryJournalName = "journal.json"

const maximumRecoveryJournalBytes = 1 << 20

var transactionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

type recoveryAdmission struct {
	snapshot durablefs.TreeSnapshot
}

func inspectRecovery(paths Paths) (recoveryAdmission, error) {
	snapshot, err := observeRecovery(paths)
	if err != nil || recoveryRetained(snapshot) {
		return recoveryAdmission{}, recoveryPending(paths, snapshot)
	}
	return recoveryAdmission{snapshot: snapshot}, nil
}

func recoveryLocation(paths Paths) (string, string) {
	base := paths.ManagedRoot
	relative := filepath.ToSlash(filepath.Join(taskrailConfigDir, "runtime", "transactions"))
	if paths.GitCommonDir != "" {
		base = paths.GitCommonDir
		relative = "taskrail/transactions"
	}
	return base, relative

}

func observeRecovery(paths Paths) (durablefs.TreeSnapshot, error) {
	base, relative := recoveryLocation(paths)
	return durablefs.ObserveTree(base, relative)
}

func recoveryRetained(snapshot durablefs.TreeSnapshot) bool {
	return snapshot.Present && len(snapshot.Entries) != 0
}

func recoveryPending(paths Paths, snapshot durablefs.TreeSnapshot) error {
	failure := MachineFailure{Code: MachineCodeRecoveryPending}
	if recovery := canonicalRecovery(paths, snapshot); recovery != nil {
		failure.Recovery = recovery
	}
	return WithMachineFailure(failure, fmt.Errorf("repository recovery is pending"))
}

func canonicalRecovery(paths Paths, snapshot durablefs.TreeSnapshot) *MachineRecoveryRef {
	if !recoveryRetained(snapshot) {
		return nil
	}
	top := ""
	journalPath := ""
	var journalSnapshot durablefs.Snapshot
	for _, entry := range snapshot.Entries {
		parts := strings.Split(entry.Path, "/")
		if len(parts) == 0 || !transactionIDPattern.MatchString(parts[0]) {
			return nil
		}
		if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			return nil
		}
		if entry.Path == top+"/"+recoveryJournalName && !entry.Directory {
			journalPath = entry.Path
			journalSnapshot = entry.Snapshot
		}
	}
	if journalPath == "" {
		return nil
	}
	base, relative := recoveryLocation(paths)
	journal, observed, err := durablefs.ReadFile(base, relative+"/"+journalPath, maximumRecoveryJournalBytes)
	if err != nil || observed != journalSnapshot {
		return nil
	}
	latest, err := observeRecovery(paths)
	if err != nil || !snapshot.Same(latest) {
		return nil
	}
	if err := checkDocumentFraming(journal); err != nil {
		return nil
	}
	object, err := strictObject(journal, "recovery journal")
	if err != nil || exactMembers(object, "recovery journal", []string{"transaction_id", "command", "phase"}) != nil {
		return nil
	}
	transactionID, transactionErr := stringMember(object, "recovery journal", "transaction_id")
	command, commandErr := stringMember(object, "recovery journal", "command")
	phase, phaseErr := stringMember(object, "recovery journal", "phase")
	if transactionErr != nil || commandErr != nil || phaseErr != nil || transactionID != top ||
		!canonicalRecoveryCommand(command) || !canonicalRecoveryPhase(phase) {
		return nil
	}
	return &MachineRecoveryRef{TransactionID: transactionID, Command: command, Phase: phase}
}

func canonicalRecoveryCommand(command string) bool {
	return isCanonicalCommandPath(command)
}

func canonicalRecoveryPhase(phase string) bool {
	for _, allowed := range machineRecoveryPhases {
		if phase == allowed {
			return true
		}
	}
	return false
}
