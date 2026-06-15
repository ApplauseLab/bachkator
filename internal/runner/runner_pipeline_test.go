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

func TestPipelineRunsStepsSequentially(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget("shell/render", "printf render >> order.txt"),
			"shell/apply":  shellTarget("shell/apply", "printf apply >> order.txt"),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"shell/render", "shell/apply"},
			),
		},
	}

	var out bytes.Buffer
	runner := Runner{Parallelism: 2, Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "pipeline/deploy"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "renderapply" {
		t.Fatalf("order = %q, want renderapply", got)
	}
	if !strings.Contains(out.String(), "[pipeline/deploy] pipeline: shell/render -> shell/apply") {
		t.Fatalf("expected pipeline output, got %q", out.String())
	}
}

func TestPipelineRunsNestedPipelineStepSequentially(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget("shell/render", "printf render >> order.txt"),
			"shell/apply":  shellTarget("shell/apply", "printf apply >> order.txt"),
			"shell/smoke":  shellTarget("shell/smoke", "printf smoke >> order.txt"),
			"pipeline/release": pipelineTarget(
				"pipeline/release",
				[]string{"shell/render", "shell/apply"},
			),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"pipeline/release", "shell/smoke"},
			),
		},
	}

	var out bytes.Buffer
	runner := Runner{Parallelism: 2, Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "pipeline/deploy"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "renderapplysmoke" {
		t.Fatalf("order = %q, want renderapplysmoke", got)
	}
	got := out.String()
	outerIndex := strings.Index(got, "[pipeline/deploy] pipeline: pipeline/release -> shell/smoke")
	innerIndex := strings.Index(got, "[pipeline/release] pipeline: shell/render -> shell/apply")
	smokeIndex := strings.Index(got, "[shell/smoke] printf smoke")
	if outerIndex < 0 || innerIndex < 0 || smokeIndex < 0 || outerIndex >= innerIndex ||
		innerIndex >= smokeIndex {
		t.Fatalf("nested pipeline output order = %q", got)
	}
}

func TestPipelineSharedDependencyRunsOnceAcrossStepClosures(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/a": shellTarget("shell/a", "printf 'a\n' >> order.txt"),
			"shell/b": shellTarget("shell/b", "printf 'b\n' >> order.txt"),
			"shell/c": shellTarget(
				"shell/c",
				"printf 'c\n' >> order.txt",
				withDependsOn("shell/b"),
			),
			"shell/d": shellTarget("shell/d", "printf 'd\n' >> order.txt"),
			"shell/group-a": shellTarget(
				"shell/group-a",
				"printf 'group-a\n' >> order.txt",
				withDependsOn("shell/a", "shell/b"),
			),
			"shell/group-b": shellTarget(
				"shell/group-b",
				"printf 'group-b\n' >> order.txt",
				withDependsOn("shell/c", "shell/d"),
			),
			"pipeline/pa": pipelineTarget(
				"pipeline/pa",
				[]string{"shell/group-a"},
			),
			"pipeline/pb": pipelineTarget(
				"pipeline/pb",
				[]string{"shell/group-b"},
			),
			"pipeline/both": pipelineTarget(
				"pipeline/both",
				[]string{"pipeline/pa", "pipeline/pb"},
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Parallelism: 3, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/both",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(contents)
	if countLine(got, "b") != 1 {
		t.Fatalf("order = %q, want shell/b to run once", got)
	}
	assertSubsequence(t, got, []string{"a", "group-a", "c", "group-b"})
	assertSubsequence(t, got, []string{"b", "group-a", "c", "group-b"})
	assertSubsequence(t, got, []string{"group-a", "d", "group-b"})
}

func TestPipelineRunsGroupsAsDAGClosures(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/a": shellTarget("shell/a", "printf 'a\n' >> order.txt"),
			"shell/b": shellTarget("shell/b", "printf 'b\n' >> order.txt"),
			"shell/c": shellTarget(
				"shell/c",
				"printf 'c\n' >> order.txt",
				withDependsOn("shell/a"),
			),
			"shell/d":    shellTarget("shell/d", "printf 'd\n' >> order.txt"),
			"group/a":    groupTarget("group/a", []string{"shell/a", "shell/b"}),
			"group/b":    groupTarget("group/b", []string{"shell/c", "shell/d"}),
			"pipeline/a": pipelineTarget("pipeline/a", []string{"group/a"}),
			"pipeline/b": pipelineTarget("pipeline/b", []string{"group/b"}),
			"pipeline/both": pipelineTarget(
				"pipeline/both",
				[]string{"pipeline/a", "pipeline/b"},
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Parallelism: 3, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/both",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(contents)
	if countLine(got, "a") != 1 {
		t.Fatalf("order = %q, want shell/a to run once", got)
	}
	assertSubsequence(t, got, []string{"a", "c"})
	if strings.Index(got, "c\n") < strings.Index(got, "a\n") {
		t.Fatalf("order = %q, want c after a", got)
	}
	stdout := out.String()
	for _, want := range []string{
		"[group/a] group: shell/a, shell/b",
		"[pipeline/a] pipeline: group/a",
		"[group/b] group: shell/c, shell/d",
		"[pipeline/b] pipeline: group/b",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
}

func TestPipelineStopsAtFirstFailedStep(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget("shell/render", "printf render >> order.txt"),
			"shell/apply":  shellTarget("shell/apply", "printf apply >> order.txt; exit 1"),
			"shell/smoke":  shellTarget("shell/smoke", "printf smoke >> order.txt"),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"shell/render", "shell/apply", "shell/smoke"},
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/deploy",
	)
	if err == nil {
		t.Fatal("expected pipeline step failure")
	}
	contents, readErr := os.ReadFile(filepath.Join(dir, "order.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := string(contents); got != "renderapply" {
		t.Fatalf("order = %q, want renderapply", got)
	}
}

func TestPipelineTimeoutWrapsStepExecution(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/slow": shellTarget("shell/slow", "sleep 0.2"),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"shell/slow"},
				withTimeout(20*time.Millisecond),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/deploy",
	)
	if err == nil || !strings.Contains(err.Error(), `target "pipeline/deploy" timed out`) {
		t.Fatalf("error = %v, want pipeline timeout", err)
	}
}

func TestPipelineDependsOnGatesScopeStart(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/prepare": shellTarget(
				"shell/prepare",
				"sleep 0.1; touch prepared; printf 'prepare\n' >> order.txt",
			),
			"shell/build": shellTarget(
				"shell/build",
				"test -f prepared; printf 'build\n' >> order.txt",
			),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"shell/build"},
				withDependsOn("shell/prepare"),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/deploy",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	assertSubsequence(t, string(contents), []string{"prepare", "build"})
}

func TestNestedPipelineTimeoutUsesInnerScope(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/slow": shellTarget("shell/slow", "sleep 0.2"),
			"pipeline/inner": pipelineTarget(
				"pipeline/inner",
				[]string{"shell/slow"},
				withTimeout(20*time.Millisecond),
			),
			"pipeline/outer": pipelineTarget(
				"pipeline/outer",
				[]string{"pipeline/inner"},
				withTimeout(time.Second),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/outer",
	)
	if err == nil || !strings.Contains(err.Error(), `target "pipeline/inner" timed out`) {
		t.Fatalf("error = %v, want inner pipeline timeout", err)
	}
}

func TestPipelineDryRunPrintsStepsInOrder(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget("shell/render", "printf render"),
			"shell/apply":  shellTarget("shell/apply", "printf apply"),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"shell/render", "shell/apply"},
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{DryRun: true, Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/deploy",
	); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	pipelineIndex := strings.Index(got, "[pipeline/deploy] pipeline: shell/render -> shell/apply")
	renderIndex := strings.Index(got, "[shell/render] printf render")
	applyIndex := strings.Index(got, "[shell/apply] printf apply")
	if pipelineIndex < 0 || renderIndex < 0 || applyIndex < 0 || pipelineIndex >= renderIndex ||
		renderIndex >= applyIndex {
		t.Fatalf("dry-run output order = %q", got)
	}
}

func TestPipelineRunsWhenUsedAsDependency(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget("shell/render", "printf render >> order.txt"),
			"shell/apply":  shellTarget("shell/apply", "printf apply >> order.txt"),
			"pipeline/deploy": pipelineTarget(
				"pipeline/deploy",
				[]string{"shell/render", "shell/apply"},
			),
			"shell/notify": shellTarget(
				"shell/notify",
				"printf notify >> order.txt",
				withDependsOn("pipeline/deploy"),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/notify",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "order.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "renderapplynotify" {
		t.Fatalf("order = %q, want renderapplynotify", got)
	}
}

func countLine(contents string, want string) int {
	count := 0
	for _, line := range strings.Split(contents, "\n") {
		if line == want {
			count++
		}
	}
	return count
}

func assertSubsequence(t *testing.T, contents string, want []string) {
	t.Helper()
	start := 0
	for _, item := range want {
		index := strings.Index(contents[start:], item+"\n")
		if index < 0 {
			t.Fatalf("order = %q, want subsequence %#v", contents, want)
		}
		start += index + len(item) + 1
	}
}
