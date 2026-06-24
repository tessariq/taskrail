package taskrail

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func sampleImportDraft() ImportDraft {
	return ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "planning",
		Source:        "notes.md",
		Tasks: []TaskDraft{
			{
				Key:          "auth",
				Title:        "Add auth middleware",
				SpecRef:      "specs/v0.2.0.md#taskrail-import",
				Priority:     "high",
				Dependencies: []string{"T-027"},
				Body:         "## Description\n\nWire up auth.\n",
			},
			{
				Key:          "auth-tests",
				Title:        "Cover auth middleware",
				Dependencies: []string{"auth"},
			},
		},
		SpecSections: []SpecSectionDraft{
			{Heading: "Auth", Body: "Describe auth surface."},
		},
	}
}

func TestImportDraftRoundTrip(t *testing.T) {
	t.Parallel()
	original := sampleImportDraft()

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := ParseImportDraft(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !reflect.DeepEqual(original, parsed) {
		t.Fatalf("round-trip mismatch:\n original=%+v\n parsed=  %+v", original, parsed)
	}
}

func TestParseImportDraftRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	_, err := ParseImportDraft([]byte(`{"schema_version":1,"target":"tasks","bogus":true}`))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestParseImportDraftRejectsTrailingContent(t *testing.T) {
	t.Parallel()
	_, err := ParseImportDraft([]byte(`{"schema_version":1,"target":"tasks"}{"extra":true}`))
	if err == nil {
		t.Fatal("expected error for trailing content after the draft object, got nil")
	}
}

func TestValidateImportDraftAcceptsSample(t *testing.T) {
	t.Parallel()
	if violations := ValidateImportDraft(sampleImportDraft()); len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestValidateImportDraftReportsViolations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mutate   func(*ImportDraft)
		contains string
	}{
		{
			name:     "wrong schema version",
			mutate:   func(d *ImportDraft) { d.SchemaVersion = 99 },
			contains: "schema_version",
		},
		{
			name:     "invalid target",
			mutate:   func(d *ImportDraft) { d.Target = "everything" },
			contains: "target",
		},
		{
			name:     "empty payload",
			mutate:   func(d *ImportDraft) { d.Tasks = nil; d.SpecSections = nil },
			contains: "at least one task or spec section",
		},
		{
			name:     "missing title",
			mutate:   func(d *ImportDraft) { d.Tasks[0].Title = "  " },
			contains: "missing title",
		},
		{
			name:     "invalid priority",
			mutate:   func(d *ImportDraft) { d.Tasks[0].Priority = "urgent" },
			contains: "invalid priority",
		},
		{
			name:     "malformed spec_ref",
			mutate:   func(d *ImportDraft) { d.Tasks[0].SpecRef = "specs/v0.2.0.md" },
			contains: "invalid spec_ref",
		},
		{
			name:     "empty spec_ref anchor",
			mutate:   func(d *ImportDraft) { d.Tasks[0].SpecRef = "specs/v0.2.0.md#" },
			contains: "invalid spec_ref",
		},
		{
			name:     "empty spec_ref path",
			mutate:   func(d *ImportDraft) { d.Tasks[0].SpecRef = "#taskrail-import" },
			contains: "invalid spec_ref",
		},
		{
			name:     "traversal spec_ref",
			mutate:   func(d *ImportDraft) { d.Tasks[0].SpecRef = "../../etc/passwd#x" },
			contains: "invalid spec_ref",
		},
		{
			name: "whitespace-only dependency handle",
			mutate: func(d *ImportDraft) {
				d.Tasks[0].Key = "  "
				d.Tasks[1].Dependencies = []string{"  "}
			},
			contains: "unresolved dependency",
		},
		{
			name:     "self dependency",
			mutate:   func(d *ImportDraft) { d.Tasks[0].Dependencies = []string{"auth"} },
			contains: "cannot depend on itself",
		},
		{
			name:     "unresolved dependency",
			mutate:   func(d *ImportDraft) { d.Tasks[0].Dependencies = []string{"nope"} },
			contains: "unresolved dependency",
		},
		{
			name:     "duplicate key",
			mutate:   func(d *ImportDraft) { d.Tasks[1].Key = "auth" },
			contains: "duplicate task draft key",
		},
		{
			name:     "spec section missing heading",
			mutate:   func(d *ImportDraft) { d.SpecSections[0].Heading = "" },
			contains: "missing heading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			draft := sampleImportDraft()
			tt.mutate(&draft)
			violations := ValidateImportDraft(draft)
			hasViolation := slices.ContainsFunc(violations, func(v string) bool {
				return strings.Contains(v, tt.contains)
			})
			if !hasViolation {
				t.Fatalf("expected a violation containing %q, got %v", tt.contains, violations)
			}
		})
	}
}
