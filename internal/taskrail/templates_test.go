package taskrail

import "testing"

func TestNativeTaskBodyRenderersHaveGoldenOutputs(t *testing.T) {
	const guidance = "TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value."
	const acceptance = "- TODO: define observable acceptance criteria for the outcome."
	const verification = "- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.\n- TODO: record later evidence paths after verification."

	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "normal scaffold warns against an oversized bundle",
			got:  renderNewTaskBody("T-001", "Oversized bundle", ""),
			want: "# T-001 Oversized bundle\n\n## Description\n\nTODO: describe the outcome's invariant and relevant spec section.\n\n" + guidance + "\n\n## Acceptance\n\n" + acceptance + "\n\n## Verification Notes\n\n" + verification + "\n\n## Implementation Notes\n",
		},
		{
			name: "implementation follow-up warns against a fragmented outcome",
			got:  renderNewTaskBody("T-002", "Fragmented outcome", "Follow-up derived from T-001's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery."),
			want: "# T-002 Fragmented outcome\n\n## Description\n\nTODO: describe the outcome's invariant and relevant spec section.\n\nFollow-up derived from T-001's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.\n\n" + guidance + "\n\n## Acceptance\n\n" + acceptance + "\n\n## Verification Notes\n\n" + verification + "\n\n## Implementation Notes\n",
		},
		{
			name: "verification follow-up owns the deferred outcome",
			got:  renderFollowupTaskBody("T-003", "Deferred outcome", "Deliver the deferred boundary behavior as one integrated result.", "T-001"),
			want: "# T-003 Deferred outcome\n\n## Description\n\nDeferred independently meaningful outcome: Deliver the deferred boundary behavior as one integrated result.\n\nThis task owns integrated delivery of the deferred outcome and its invariant after T-001's verification.\n\n" + guidance + "\n\n## Acceptance\n\n" + acceptance + "\n\n## Verification Notes\n\n" + verification + "\n\n## Implementation Notes\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("body =\n%s\nwant:\n%s", tc.got, tc.want)
			}
		})
	}
}
