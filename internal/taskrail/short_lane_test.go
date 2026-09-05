package taskrail

import "testing"

// skipDurableSkillPublication skips a test whose cost is dominated by publishing
// the embedded skill set through a durable transaction. Every barrier in that
// path is a device flush — F_FULLFSYNC on darwin (internal/durablefs) — so these
// tests are I/O-bound, do not benefit from parallelism, and dominate the local
// pre-push lane.
//
// They still run in CI, which is the authoritative gate: this only keeps
// `task test:short` fast enough that contributors do not learn to skip hooks.
func skipDurableSkillPublication(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("durable skill publication is I/O-bound; run without -short (CI runs it)")
	}
}
