package taskrail

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriterTransactionMatrixCoversMachineInventory(t *testing.T) {
	entries := WriterTransactionMatrix()
	covered := make(map[string]bool)
	for _, entry := range entries {
		covered[entry.Command+"\x00"+string(entry.Surface)] = true
	}
	for _, command := range MachineCommandInventory() {
		key := command.Command + "\x00" + string(command.Surface)
		if !covered[key] {
			t.Errorf("machine command %q has no writer classification", command.CompanionRow)
		}
	}
}

func TestWriterTransactionMatrixRejectsInvalidEntries(t *testing.T) {
	entries := WriterTransactionMatrix()
	if err := validateWriterTransactionMatrix(entries); err != nil {
		t.Fatalf("valid matrix: %v", err)
	}

	tests := []struct {
		name   string
		mutate func([]WriterTransaction)
	}{
		{
			name: "duplicate ownership",
			mutate: func(entries []WriterTransaction) {
				entries[1].Owner = entries[0].Owner
			},
		},
		{
			name: "unregistered sink",
			mutate: func(entries []WriterTransaction) {
				entries[0].Publishes = append(entries[0].Publishes, "unregistered sink")
			},
		},
		{
			name: "wrong durability annotation",
			mutate: func(entries []WriterTransaction) {
				for i := range entries {
					if entries[i].Durability == WriterDurabilityNormal {
						entries[i].Durability = WriterDurabilityDurable
						return
					}
				}
				t.Fatal("matrix has no normal writer")
			},
		},
		{
			name: "durable flow without phase evidence",
			mutate: func(entries []WriterTransaction) {
				for i := range entries {
					if entries[i].Durability == WriterDurabilityDurable {
						entries[i].Recovery = WriterRecoveryNone
						return
					}
				}
				t.Fatal("matrix has no durable writer")
			},
		},
		{
			name: "review publisher omits prompt snapshot",
			mutate: func(entries []WriterTransaction) {
				for i := range entries {
					if entries[i].Owner == "review publish task" {
						entries[i].Consumes = []string{"proposal", "task", "spec"}
						return
					}
				}
				t.Fatal("matrix has no task-review publisher")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := WriterTransactionMatrix()
			tt.mutate(candidate)
			if err := validateWriterTransactionMatrix(candidate); err == nil {
				t.Fatal("invalid matrix was accepted")
			}
		})
	}
}

func TestWriterTransactionMatrixMatchesPublicationEntrypoints(t *testing.T) {
	declared := map[string]WriterDurability{}
	for _, entry := range WriterTransactionMatrix() {
		for _, implementation := range entry.Implementation {
			if previous, exists := declared[implementation]; exists && previous != entry.Durability {
				t.Fatalf("implementation %q has conflicting matrix classes %q and %q", implementation, previous, entry.Durability)
			}
			declared[implementation] = entry.Durability
		}
	}

	actual := publicationEntrypoints(t)
	for implementation, durability := range actual {
		if got, exists := declared[implementation]; !exists {
			t.Errorf("publication entrypoint %q is not classified", implementation)
		} else if got != durability {
			t.Errorf("publication entrypoint %q has matrix class %q, want %q", implementation, got, durability)
		}
	}
	for implementation := range declared {
		if _, exists := actual[implementation]; !exists {
			t.Errorf("matrix implementation %q is not a current publication entrypoint", implementation)
		}
	}
}

func publicationEntrypoints(t *testing.T) map[string]WriterDurability {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate writer matrix test source")
	}
	directory := filepath.Dir(testFile)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read taskrail source directory: %v", err)
	}
	publication := map[string]WriterDurability{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				packageName, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				durability, tracked := publicationCallDurability(packageName.Name, selector.Sel.Name)
				if tracked {
					key := packageName.Name + "." + selector.Sel.Name + ":" + function.Name.Name
					publication[key] = durability
				}
				return true
			})
		}
	}
	return publication
}

func publicationCallDurability(packageName, function string) (WriterDurability, bool) {
	switch packageName + "." + function {
	case "repotx.Commit":
		return WriterDurabilityNormal, true
	case "durabletx.Run":
		return WriterDurabilityDurable, true
	case "reviewdir.Publish":
		return WriterDurabilityDirectory, true
	default:
		return "", false
	}
}
