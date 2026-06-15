package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerBlocksRequiresConfirmationWithoutYes(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf deploy > deployed.txt",
				withRisk(true, false, true),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "shell/deploy")
	if err == nil ||
		!strings.Contains(err.Error(), `target "shell/deploy" requires confirmation`) ||
		!strings.Contains(err.Error(), "--dry-run") ||
		!strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %v, want confirmation guard with hints", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "deployed.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("guarded target wrote output, stat error = %v", statErr)
	}
}

func TestRunnerAllowsRequiresConfirmationWithYes(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf deploy > deployed.txt",
				withRisk(false, false, true),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Yes: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/deploy",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "deployed.txt")); err != nil {
		t.Fatalf("expected confirmed target output: %v", err)
	}
}

func TestRunnerDryRunBypassesRequiresConfirmation(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf deploy > deployed.txt",
				withRisk(false, false, true),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/deploy",
	); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "deployed.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("dry-run wrote output, stat error = %v", statErr)
	}
}

func TestRunnerBlocksInheritedConfirmationFromDependency(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/apply": shellTarget(
				"shell/apply",
				"printf apply > applied.txt",
				withRisk(false, true, true),
			),
			"shell/deploy": shellTarget("shell/deploy", "", withDependsOn("shell/apply")),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "shell/deploy")
	if err == nil ||
		!strings.Contains(err.Error(), `target "shell/deploy" requires confirmation`) ||
		!strings.Contains(err.Error(), "destructive") {
		t.Fatalf("error = %v, want inherited confirmation guard", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "applied.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("guarded dependency wrote output, stat error = %v", statErr)
	}
}

func TestRunnerBlocksInheritedConfirmationFromPipelineStep(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget("shell/render", "printf render > render.txt"),
			"shell/apply": shellTarget(
				"shell/apply",
				"printf apply > applied.txt",
				withRisk(true, false, true),
			),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"shell/render", "shell/apply"},
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/deploy",
	)
	if err == nil ||
		!strings.Contains(err.Error(), `target "pipeline/deploy" requires confirmation`) ||
		!strings.Contains(err.Error(), "remote") {
		t.Fatalf("error = %v, want inherited pipeline confirmation guard", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "render.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("guarded pipeline started steps, stat error = %v", statErr)
	}
}
