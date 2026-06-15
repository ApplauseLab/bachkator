package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	statestore "github.com/applauselab/bachkator/internal/state"
)

func TestGroupDependsOnGatesMembers(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/prepare": shellTarget(
				"shell/prepare",
				"sleep 0.1; touch prepared; printf 'prepare\n' >> order.txt",
			),
			"shell/check": shellTarget(
				"shell/check",
				"test -f prepared; printf 'check\n' >> order.txt",
			),
			"group/ci": groupTarget(
				"group/ci",
				[]string{"shell/check"},
				withDependsOn("shell/prepare"),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"group/ci",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	assertSubsequence(t, string(contents), []string{"prepare", "check"})
}

func TestGroupTimeoutWrapsMemberExecution(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/slow": shellTarget("shell/slow", "sleep 0.2"),
			"group/ci": groupTarget(
				"group/ci",
				[]string{"shell/slow"},
				withTimeout(20*time.Millisecond),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"group/ci",
	)
	if err == nil || !strings.Contains(err.Error(), `target "group/ci" timed out`) {
		t.Fatalf("error = %v, want group timeout", err)
	}
}

func TestGroupTimeoutWrapsMemberDependencyExecution(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/slow-setup": shellTarget("shell/slow-setup", "sleep 0.2"),
			"shell/work": shellTarget(
				"shell/work",
				"printf 'work\n' >> order.txt",
				withDependsOn("shell/slow-setup"),
			),
			"group/ci": groupTarget(
				"group/ci",
				[]string{"shell/work"},
				withTimeout(20*time.Millisecond),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"group/ci",
	)
	if err == nil || !strings.Contains(err.Error(), `target "group/ci" timed out`) {
		t.Fatalf("error = %v, want group timeout", err)
	}
}

func TestOuterPipelineTimeoutWrapsNestedGroupTarget(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/slow": shellTarget("shell/slow", "sleep 0.2"),
			"group/inner": groupTarget(
				"group/inner",
				[]string{"shell/slow"},
			),
			"pipeline/outer": pipelineTarget(
				"pipeline/outer",
				[]string{"group/inner"},
				withTimeout(20*time.Millisecond),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/outer",
	)
	if err == nil || !strings.Contains(err.Error(), `target "pipeline/outer" timed out`) {
		t.Fatalf("error = %v, want outer pipeline timeout", err)
	}
}

func TestCompositeDependsOnAndMemberDependencyOverlapDoesNotCycle(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/setup": shellTarget(
				"shell/setup",
				"printf 'setup\n' >> order.txt",
			),
			"shell/work": shellTarget(
				"shell/work",
				"printf 'work\n' >> order.txt",
				withDependsOn("shell/setup"),
			),
			"group/ci": groupTarget(
				"group/ci",
				[]string{"shell/work"},
				withDependsOn("shell/setup"),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"group/ci",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(contents)
	if countLine(got, "setup") != 1 {
		t.Fatalf("order = %q, want setup once", got)
	}
	assertSubsequence(t, got, []string{"setup", "work"})
}

func TestCompositeTargetRecordsFailureWhenMemberFails(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/fail":      shellTarget("shell/fail", "exit 1"),
			"pipeline/deploy": pipelineTarget("pipeline/deploy", []string{"shell/fail"}),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/deploy",
	)
	if err == nil {
		t.Fatal("expected pipeline failure")
	}
	runs, err := newTestStore(t, project.StatePath).ListRuns(statestore.RunQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if got := runs[0].Targets["pipeline/deploy"].Status; got != "failed" {
		t.Fatalf("pipeline/deploy status = %q, want failed", got)
	}
}

func TestNestedCompositeWithSameLockDoesNotDeadlock(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/work": shellTarget(
				"shell/work",
				"printf 'work\n' >> order.txt",
				withLock("shared"),
			),
			"group/inner": groupTarget(
				"group/inner",
				[]string{"shell/work"},
				withLock("shared"),
			),
			"pipeline/outer": pipelineTarget(
				"pipeline/outer",
				[]string{"group/inner"},
				withLock("shared"),
			),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out bytes.Buffer
	if err := (&Runner{Parallelism: 3, Stdout: &out, Stderr: &out}).Run(
		ctx,
		project,
		"pipeline/outer",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "work\n" {
		t.Fatalf("order = %q, want work", contents)
	}
}
