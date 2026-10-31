//go:build !windows

package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSkillEvalStageSealsEveryArtifactAncestor(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderSkillEvalStage(stage)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSkillEvalStage(data, in)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ArtifactAnchor == nil {
		t.Fatal("stage omitted artifact anchor")
	}
	current := in.ArtifactRoot
	for index, fact := range decoded.ArtifactAnchor.Ancestors {
		own, err := os.Lstat(current)
		if err != nil {
			t.Fatal(err)
		}
		target, err := os.Stat(current)
		if err != nil {
			t.Fatal(err)
		}
		stat := target.Sys().(*syscall.Stat_t)
		if fact.Type != uint32(own.Mode().Type()) || fact.Device != uint64(stat.Dev) || fact.Inode != uint64(stat.Ino) || own.IsDir() && !os.SameFile(own, target) {
			t.Fatalf("ancestor %d = %+v, does not match direct filesystem observation", index, fact)
		}
		if current == filepath.Dir(current) {
			if index != len(decoded.ArtifactAnchor.Ancestors)-1 {
				t.Fatal("facts extend above filesystem root")
			}
			return
		}
		current = filepath.Dir(current)
	}
	t.Fatal("facts did not reach filesystem root")
}
