package main

import (
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		wantErrs []string // substrings; nil means the check must pass
	}{
		{
			name:  "clean projection",
			input: `{"command":"repair","result":{"applied":false,"changes":[],"validation":{"valid":true,"violations":[]}}}`,
		},
		{
			name:     "frontmatter drift names the field and the remedy",
			input:    `{"command":"repair","result":{"changes":[{"field":"status_summary","from":"active","to":"idle"}],"validation":{"valid":true,"violations":[]}}}`,
			wantErrs: []string{"status_summary", `"active"`, `"idle"`, "taskrail repair --apply", "never be hand-edited"},
		},
		{
			name:     "body drift alone fails",
			input:    `{"command":"repair","result":{"changes":[],"body_diff":["- Ready: 3","+ Ready: 4"],"validation":{"valid":true,"violations":[]}}}`,
			wantErrs: []string{"rendered body", "+ Ready: 4", "taskrail repair --apply"},
		},
		{
			name:     "invalid tree fails without drift",
			input:    `{"command":"repair","result":{"changes":[],"validation":{"valid":false,"violations":["T-001 missing spec_ref"]}}}`,
			wantErrs: []string{"validation fails", "T-001 missing spec_ref"},
		},
		{
			name:     "missing validation is not read as valid",
			input:    `{"command":"repair","result":{"changes":[]}}`,
			wantErrs: []string{"no validation result"},
		},
		{
			name:     "another command's envelope is rejected",
			input:    `{"command":"validate","result":{"valid":true}}`,
			wantErrs: []string{"expected a repair envelope"},
		},
		{
			name:     "empty stdin fails",
			input:    ``,
			wantErrs: []string{"read `taskrail repair --json` output"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := run(strings.NewReader(tc.input))
			if tc.wantErrs == nil {
				if err != nil {
					t.Fatalf("run: unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("run: expected an error")
			}
			for _, want := range tc.wantErrs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}
