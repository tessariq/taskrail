package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/tessariq/taskrail/internal/taskrail"
)

func main() {
	manifestOnly := flag.Bool("manifest", false, "print the generated manifest without running tests")
	flag.Parse()

	if *manifestOnly {
		manifest, err := taskrail.WorkflowContractTestSurfaceManifest()
		if err != nil {
			fail(err)
		}
		emit(manifest)
		return
	}

	root, err := os.Getwd()
	if err != nil {
		fail(fmt.Errorf("determine repository root: %w", err))
	}
	results, err := taskrail.RunWorkflowContractSuites(context.Background(), root)
	if err != nil {
		fail(err)
	}
	emit(results)
}

func emit(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fail(fmt.Errorf("write result: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "workflow-contract:", err)
	os.Exit(1)
}
