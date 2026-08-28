package main

import (
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

func TestRenderLocalPromoteTextIncludesRemovedExclusions(t *testing.T) {
	text := renderLocalPromoteText(taskrail.LocalPromoteResult{
		RemovedExclusions: []string{".agents/skills/taskrail-repair"},
	})
	if !strings.Contains(text, "remove exclusion: .agents/skills/taskrail-repair") {
		t.Fatalf("promotion text = %q", text)
	}
}
