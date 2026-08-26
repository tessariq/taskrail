package durabletx

import (
	"context"
	"fmt"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
)

// The fence member's two-stage publication. A fenced transaction publishes its
// fence bytes after the originals are recorded durably and before any other
// semantic byte changes; the fence member's final candidate then publishes as
// the transaction's last semantic operation, after post-publication validation
// (specs/v0.5.0.md#layout-compatibility-and-upgrade). Recovery derives every
// intermediate state from the journal plus these recorded byte states, so an
// interruption at any point still leaves exactly one mechanical action.

// publishFenceMembers publishes each fence member's intermediate bytes. The
// journal already announced the fence-published phase, so an interruption
// before these bytes land leaves an all-original set that recovery clears.
func publishFenceMembers(store *store, entries []*transactionEntry) error {
	for _, entry := range entries {
		if entry.manifest.Fence == nil {
			continue
		}
		current, _, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil || !sameOriginal(entry, current) {
			return fmt.Errorf("%s changed before fence publication: %w", entry.manifest.Reported, err)
		}
		if err := put(store, entry, current, entry.fence, entry.manifest.Fence.mode()); err != nil {
			return err
		}
	}
	return nil
}

// recheckAfterFence proves every source snapshot one more time after the fence
// bytes are durable: the fence member must hold its fence bytes and everything
// else its original, so publication begins from a re-proven set.
func recheckAfterFence(store *store, entries []*transactionEntry) error {
	current, err := observeEntries(store, entries)
	if err != nil {
		return err
	}
	for i, entry := range entries {
		if entry.manifest.Fence != nil {
			if !holdsFence(entry, current[i]) {
				return fmt.Errorf("%s does not hold its published fence bytes", entry.manifest.Reported)
			}
			continue
		}
		if !sameOriginal(entry, current[i]) {
			return fmt.Errorf("%s changed since preparation", entry.manifest.Reported)
		}
	}
	return nil
}

// publishFenceFinals performs the transaction's last semantic operation:
// replacing each fence member's intermediate bytes with its final candidate.
// A fence member already holding its final bytes (a retried recovery, or an
// interruption after this replace) is complete.
func publishFenceFinals(ctx context.Context, store *store, entries []*transactionEntry) error {
	for _, entry := range entries {
		if entry.manifest.Fence == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		current, _, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil {
			return err
		}
		if holdsCandidate(entry, current) {
			continue
		}
		if !holdsFence(entry, current) {
			return fmt.Errorf("%s changed before final publication", entry.manifest.Reported)
		}
		if err := put(store, entry, current, entry.candidate, entry.manifest.Candidate.mode()); err != nil {
			return err
		}
	}
	return nil
}

// completeFenceFinals is publishFenceFinals over retained evidence, for the
// recovery engine's accept action: it finishes an interrupted transaction from
// the final bytes the manifest retained, never from re-derived content.
func completeFenceFinals(ctx context.Context, store *store, entries []*transactionEntry) error {
	for _, entry := range entries {
		if entry.manifest.Fence == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		current, _, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil {
			return err
		}
		if holdsCandidate(entry, current) {
			continue
		}
		if !holdsFence(entry, current) {
			return fmt.Errorf("%s changed before completing the retained candidate", entry.manifest.Reported)
		}
		if entry.candidate == nil {
			return fmt.Errorf("%s retains no final bytes to complete", entry.manifest.Reported)
		}
		if err := put(store, entry, current, entry.candidate, entry.manifest.Candidate.mode()); err != nil {
			return err
		}
	}
	return nil
}

// holdsFence reports whether the member currently holds its recorded fence
// bytes through the recorded path identities.
func holdsFence(entry *transactionEntry, current observation) bool {
	return entry.manifest.Fence != nil && current.present && entry.manifest.Fence.holds(current.snapshot) &&
		identityPrefix(entry.manifest.Ancestors, current.ancestors)
}

// candidateComplete is the whole-set state a validated fenced transaction
// reaches before its fence member publishes the final candidate: every one-stage
// member holds its candidate, every consumed path its original, and the fence
// member its fence bytes or — once the final publication ran — its candidate.
func candidateComplete(entries []*transactionEntry, current []observation) bool {
	for i, entry := range entries {
		switch {
		case entry.manifest.Fence != nil:
			if !holdsCandidate(entry, current[i]) && !holdsFence(entry, current[i]) {
				return false
			}
		case entry.manifest.Published:
			if !holdsCandidate(entry, current[i]) {
				return false
			}
		case !sameOriginal(entry, current[i]):
			return false
		}
	}
	return true
}

// onlyFenceWritten reports whether every deviation from the recorded originals
// is a fence member holding its fence bytes, which is the whole published set
// at the fence-published phase.
func onlyFenceWritten(entries []*transactionEntry, current []observation) bool {
	for i, entry := range entries {
		if sameOriginal(entry, current[i]) || holdsFence(entry, current[i]) {
			continue
		}
		return false
	}
	return true
}

// RetainedCandidate returns the retained final candidate bytes one fenced
// member recorded, so the owning command's recovery validator can prove the
// accepted candidate is exactly what its writer decided — mechanically, without
// re-deriving semantic content. It refuses a member that published in one
// stage, whose final bytes are the repository's rather than retained evidence.
func RetainedCandidate(repo repolock.Repository, transactionID string, kind PathKind, reported string) ([]byte, error) {
	if !transactionIDPattern.MatchString(transactionID) {
		return nil, fmt.Errorf("transaction id %q is not a lower-case 32-hex id", transactionID)
	}
	base, relative := transactionsPath(repo)
	data, _, err := durablefs.ReadFile(base, relative+"/"+transactionID+"/"+manifestName, maximumJournalBytes)
	if err != nil {
		return nil, err
	}
	var saved manifest
	if err := decodeDocument(data, &saved); err != nil {
		return nil, err
	}
	for i, member := range saved.Members {
		if member.Reported != reported || member.Kind != kind || member.Fence == nil {
			continue
		}
		final, _, err := durablefs.ReadFile(base,
			fmt.Sprintf("%s/%s/%08d", relative+"/"+transactionID, finalsDirName, i), maximumJournalBytes)
		if err != nil {
			return nil, err
		}
		if member.Candidate == nil || digest(final) != member.Candidate.SHA256 {
			return nil, fmt.Errorf("retained final for %s disagrees with the manifest", reported)
		}
		return final, nil
	}
	return nil, fmt.Errorf("transaction %s retains no fenced final for %s %s", transactionID, kind, reported)
}

// RetainedOriginal returns the exact original bytes recorded for one member. A
// false present value means the transaction recorded absence rather than bytes.
func RetainedOriginal(repo repolock.Repository, transactionID string, kind PathKind, reported string) ([]byte, bool, error) {
	if !transactionIDPattern.MatchString(transactionID) {
		return nil, false, fmt.Errorf("transaction id %q is not a lower-case 32-hex id", transactionID)
	}
	base, relative := transactionsPath(repo)
	data, _, err := durablefs.ReadFile(base, relative+"/"+transactionID+"/"+manifestName, maximumJournalBytes)
	if err != nil {
		return nil, false, err
	}
	var saved manifest
	if err := decodeDocument(data, &saved); err != nil {
		return nil, false, err
	}
	for i, member := range saved.Members {
		if member.Reported != reported || member.Kind != kind {
			continue
		}
		if member.Original == nil {
			return nil, false, nil
		}
		original, _, err := durablefs.ReadFile(base,
			fmt.Sprintf("%s/%s/%08d", relative+"/"+transactionID, originalsDirName, i), maximumJournalBytes)
		if err != nil {
			return nil, false, err
		}
		if digest(original) != member.Original.SHA256 {
			return nil, false, fmt.Errorf("retained original for %s disagrees with the manifest", reported)
		}
		return original, true, nil
	}
	return nil, false, fmt.Errorf("transaction %s retains no original for %s %s", transactionID, kind, reported)
}
