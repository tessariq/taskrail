package main

import "testing"

func TestParseRecommendation(t *testing.T) {
	cases := []struct {
		name    string
		details string
		want    string
		wantErr bool
	}{
		{
			name:    "own line",
			details: "checks green.\nfollow-up recommendation: run - separate spec area\n",
			want:    "follow-up recommendation: run - separate spec area",
		},
		{
			// The shape `taskrail verify --details` emits: one paragraph, so the
			// marker follows prose instead of starting a physical line.
			name:    "inline after prose",
			details: "Reviews: 1 finding, separate-followup. follow-up recommendation: hold - operator review required",
			want:    "follow-up recommendation: hold - operator review required",
		},
		{
			name:    "trailing whitespace trimmed",
			details: "follow-up recommendation: hold - operator review required   ",
			want:    "follow-up recommendation: hold - operator review required",
		},
		{name: "missing marker", details: "no advisory here", wantErr: true},
		{
			name:    "duplicate markers",
			details: "follow-up recommendation: run - one. follow-up recommendation: hold - two",
			wantErr: true,
		},
		{name: "unsupported mode", details: "follow-up recommendation: maybe - unclear", wantErr: true},
		{name: "empty rationale", details: "follow-up recommendation: run - ", wantErr: true},
		{name: "missing rationale separator", details: "follow-up recommendation: run", wantErr: true},
		{name: "empty details", details: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRecommendation(tc.details)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseRecommendation(%q) = %q, want error", tc.details, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRecommendation(%q) failed: %v", tc.details, err)
			}
			if got != tc.want {
				t.Fatalf("parseRecommendation(%q) = %q, want %q", tc.details, got, tc.want)
			}
		})
	}
}
