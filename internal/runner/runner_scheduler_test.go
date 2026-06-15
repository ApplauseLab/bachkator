package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunnerRunsReadyDependenciesInParallel(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"a": shellTarget(
				"a",
				"touch a.started; while [ ! -f b.started ]; do sleep 0.01; done",
			),
			"b": shellTarget(
				"b",
				"touch b.started; while [ ! -f a.started ]; do sleep 0.01; done",
			),
			"all": shellTarget("all", "", withDependsOn("a", "b")),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var out bytes.Buffer
	runner := Runner{Parallelism: 2, Stdout: &out, Stderr: &out}
	if err := runner.Run(ctx, project, "all"); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerHonorsParallelismLimit(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"a":   shellTarget("a", "touch a.started; sleep 0.2; touch a.done"),
			"b":   shellTarget("b", "if [ -f a.done ]; then touch b.after_a; fi; touch b.started"),
			"all": shellTarget("all", "", withDependsOn("a", "b")),
		},
	}

	var out bytes.Buffer
	runner := Runner{Parallelism: 1, Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "all"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.after_a")); err != nil {
		t.Fatalf("expected b to start after a completed with Parallelism=1: %v", err)
	}
}

func TestRunnerSerializesTargetsWithSameLock(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"a": shellTarget(
				"a",
				"test ! -f postgres.lock; touch postgres.lock; sleep 0.1; rm postgres.lock",
				withLock("postgres"),
			),
			"b": shellTarget(
				"b",
				"test ! -f postgres.lock; touch postgres.lock; sleep 0.1; rm postgres.lock",
				withLock("postgres"),
			),
			"all": shellTarget("all", "", withDependsOn("a", "b")),
		},
	}

	var out bytes.Buffer
	runner := Runner{Parallelism: 2, Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "all"); err != nil {
		t.Fatal(err)
	}
}

func TestRunnerRunsDifferentLocksConcurrently(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"a": shellTarget(
				"a",
				"touch a.started; while [ ! -f b.started ]; do sleep 0.01; done",
				withLock("postgres"),
			),
			"b": shellTarget(
				"b",
				"touch b.started; while [ ! -f a.started ]; do sleep 0.01; done",
				withLock("container-builder"),
			),
			"all": shellTarget("all", "", withDependsOn("a", "b")),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var out bytes.Buffer
	runner := Runner{Parallelism: 2, Stdout: &out, Stderr: &out}
	if err := runner.Run(ctx, project, "all"); err != nil {
		t.Fatal(err)
	}
}

func TestPipelineStepsShareLocksWithSiblingTargets(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"guard": shellTarget(
				"guard",
				"test ! -f postgres.lock; touch postgres.lock; sleep 0.1; rm postgres.lock",
				withLock("postgres"),
			),
			"db-step": shellTarget(
				"db-step",
				"test ! -f postgres.lock; touch postgres.lock; sleep 0.1; rm postgres.lock",
				withLock("postgres"),
			),
			"pipeline/deploy": pipelineTarget("pipeline/deploy", []string{"db-step"}),
			"all":             shellTarget("all", "", withDependsOn("guard", "pipeline/deploy")),
		},
	}

	var out bytes.Buffer
	runner := Runner{Parallelism: 2, Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "all"); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunPrintsTargetLock(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"test-db": shellTarget("test-db", "printf db", withLock("postgres")),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"test-db",
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[test-db lock=postgres] printf db") {
		t.Fatalf("dry-run output = %q, want lock metadata", out.String())
	}
}

func TestRunnerWaitsForAllDependenciesBeforeDependent(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"a": shellTarget("a", "sleep 0.1; touch a.done"),
			"b": shellTarget("b", "sleep 0.2; touch b.done"),
			"join": shellTarget(
				"join",
				"test -f a.done && test -f b.done && touch join.done",
				withDependsOn("a", "b"),
			),
		},
	}

	var out bytes.Buffer
	runner := Runner{Parallelism: 3, Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "join"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "join.done")); err != nil {
		t.Fatalf("expected dependent to run after all prerequisites: %v", err)
	}
}

func TestAggregateTargetsRunDependenciesInDeclaredTopologicalOrder(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"first":  shellTarget("first", "printf first >> order.txt"),
			"second": shellTarget("second", "printf second >> order.txt"),
			"all":    shellTarget("all", "", withDependsOn("first", "second")),
		},
	}

	var out bytes.Buffer
	runner := Runner{Parallelism: 1, Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "all"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "firstsecond" {
		t.Fatalf("order = %q, want firstsecond", got)
	}
	if !strings.Contains(out.String(), "[all] aggregate") {
		t.Fatalf("expected aggregate output, got %q", out.String())
	}
}
