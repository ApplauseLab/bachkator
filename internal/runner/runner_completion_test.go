package runner

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunnerWithoutCompletionContractsUsesExitCode(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"ok": shellTarget("ok", "printf done"),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"ok",
	); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerSuccessWhenRequiresOutputMatch(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"release": shellTarget(
				"release",
				"printf 'Release complete'",
				withSuccess(CompletionCheck{OutputContains: "Release complete"}),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"release",
	); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerSuccessWhenFailsWithoutOutputMatch(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"release": shellTarget(
				"release",
				"printf 'not enough'",
				withSuccess(CompletionCheck{OutputContains: "Release complete"}),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "release")
	if err == nil || !strings.Contains(err.Error(), "success_when not satisfied") {
		t.Fatalf("error = %v, want success_when failure", err)
	}
}

func TestRunnerFailWhenFailsOnOutputMatch(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"deploy": shellTarget(
				"deploy",
				"printf 'ROLLBACK started'",
				withFail(CompletionCheck{OutputContains: "ROLLBACK"}),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "deploy")
	if err == nil || !strings.Contains(err.Error(), "fail_when matched") {
		t.Fatalf("error = %v, want fail_when failure", err)
	}
}

func TestRunnerSuccessWhenSupportsFileAndCommandChecks(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"build": shellTarget(
				"build",
				"mkdir -p dist && printf ok > dist/app",
				withSuccess(
					CompletionCheck{FileExists: "dist/app"},
					CompletionCheck{Command: []string{"test", "-s", "dist/app"}},
				),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerTimesOutLongRunningTarget(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"slow": shellTarget("slow", "sleep 1", withTimeout(20*time.Millisecond)),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "slow")
	if err == nil || !strings.Contains(err.Error(), `target "slow" timed out after 20ms`) {
		t.Fatalf("error = %v, want timeout", err)
	}
}
