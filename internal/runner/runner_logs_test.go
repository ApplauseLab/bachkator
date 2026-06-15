package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	statestore "github.com/applauselab/bachkator/internal/state"
)

func TestRunnerIndexesLogsOutputsRunDirectoryArtifactsAndImageTags(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget(
				"shell/render",
				`mkdir -p dist && printf manifest > "$BACH_RUN_DIRECTORY/deploy.yaml" && printf out > dist/app.txt`,
				withOutputs("dist/app.txt"),
			),
			"image/app": imageTarget("image/app", "example/app", []string{"dev"}),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/render",
	); err != nil {
		t.Fatal(err)
	}
	artifacts, err := newTestStore(t, project.StatePath).
		ListArtifacts(statestore.ArtifactQuery{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, artifact := range artifacts {
		got[artifact.Target+":"+artifact.Kind+":"+artifact.Path+":"+artifact.Value] = true
	}
	for _, want := range []string{
		"shell/render:log:",
		"shell/render:run-directory:",
		"shell/render:manifest:",
		"shell/render:artifact:dist/app.txt:",
	} {
		matched := false
		for key := range got {
			if strings.Contains(key, want) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("missing indexed artifact containing %q; got %#v", want, got)
		}
	}
}

func TestRunnerRecordsRunsAndStreamsTargetLogs(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"hello": shellTarget("hello", "printf stdout; printf stderr >&2"),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}
	if err := runner.Run(context.Background(), project, "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "[hello] printf stdout; printf stderr >&2") ||
		!strings.Contains(stdout.String(), "[hello] stdout") ||
		!strings.Contains(
			stdout.String(),
			"\ntargets: success=1 cached=0 failed=0 preflight-failed=0 quality-failed=0 dry-run=0 running=0\n",
		) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "[hello] stderr") {
		t.Fatalf("stderr = %q", stderr.String())
	}

	runs, err := newTestStore(t, project.StatePath).ListRuns(statestore.RunQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	run := runs[0]
	if run.Target != "hello" || run.Status != "success" {
		t.Fatalf("run = %#v", run)
	}
	targetRun := run.Targets["hello"]
	if targetRun.Status != "success" || targetRun.LogPath == "" {
		t.Fatalf("target run = %#v", targetRun)
	}
	logContents, err := os.ReadFile(targetRun.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logContents)
	if !strings.Contains(logText, "[hello] printf stdout; printf stderr >&2") ||
		!strings.Contains(logText, "stdout") ||
		!strings.Contains(logText, "stderr") {
		t.Fatalf("log does not contain command output: %q", logText)
	}
}

func TestRunnerLogOnlyStreamsProgressAndWritesOutputOnlyToLogs(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"hello": shellTarget("hello", "printf stdout; printf stderr >&2"),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{LogOnly: true, Stdout: &stdout, Stderr: &stderr}
	if err := runner.Run(context.Background(), project, "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		stdout.String(),
		"[hello] printf stdout; printf stderr >&2",
	) {
		t.Fatalf("stdout = %q, want target progress", stdout.String())
	}
	if strings.Contains(stdout.String(), "] stdout") ||
		strings.Contains(stdout.String(), "] stderr") {
		t.Fatalf("stdout = %q, want command output suppressed", stdout.String())
	}
	if !strings.Contains(
		stdout.String(),
		"targets: success=1 cached=0 failed=0 preflight-failed=0 quality-failed=0 dry-run=0 running=0\n",
	) {
		t.Fatalf("stdout = %q, want summary", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	runs, err := newTestStore(t, project.StatePath).ListRuns(statestore.RunQuery{})
	if err != nil {
		t.Fatal(err)
	}
	logContents, err := os.ReadFile(runs[0].Targets["hello"].LogPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logContents)
	if !strings.Contains(logText, "[hello] printf stdout; printf stderr >&2") ||
		!strings.Contains(logText, "stdout") ||
		!strings.Contains(logText, "stderr") {
		t.Fatalf("log does not contain progress and output: %q", logText)
	}
}

func TestRunnerLogOnlyStreamsQualityProgress(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"quality": shellTarget(
				"quality",
				`printf '<testsuite tests="1" failures="0" errors="0" skipped="0"><testcase classname="pkg" name="ok"/></testsuite>' > junit.xml`,
				withQualityReport(QualityReportDeclaration{
					Kind:   "test",
					Format: "junit-xml",
					Path:   "junit.xml",
				}),
			),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{LogOnly: true, Stdout: &stdout, Stderr: &stderr}
	if err := runner.Run(context.Background(), project, "quality"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		stdout.String(),
		"[quality] quality report junit.xml parsed: metrics=4 findings=1",
	) {
		t.Fatalf("stdout = %q, want streamed quality progress", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunnerQuietTargetWritesProgressAndOutputOnlyToLogs(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"hello": shellTarget("hello", "printf stdout; printf stderr >&2", withQuiet()),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}
	if err := runner.Run(context.Background(), project, "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(
		stdout.String(),
		"targets: success=1 cached=0 failed=0 preflight-failed=0 quality-failed=0 dry-run=0 running=0\n",
	) {
		t.Fatalf("stdout = %q, want summary", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	runs, err := newTestStore(t, project.StatePath).ListRuns(statestore.RunQuery{})
	if err != nil {
		t.Fatal(err)
	}
	logContents, err := os.ReadFile(runs[0].Targets["hello"].LogPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logContents)
	if !strings.Contains(logText, "[hello] printf stdout; printf stderr >&2") ||
		!strings.Contains(logText, "stdout") ||
		!strings.Contains(logText, "stderr") {
		t.Fatalf("log does not contain progress and output: %q", logText)
	}
}

func TestRunnerVerboseStreamsQuietTarget(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"hello": shellTarget("hello", "printf stdout; printf stderr >&2", withQuiet()),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Verbose: true, Stdout: &stdout, Stderr: &stderr}
	if err := runner.Run(context.Background(), project, "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "stdout") {
		t.Fatalf("stdout = %q, want command output", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stderr") {
		t.Fatalf("stderr = %q, want command output", stderr.String())
	}
}

func TestRunnerLogOnlyOverridesVerbose(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"hello": shellTarget("hello", "printf stdout; printf stderr >&2"),
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{LogOnly: true, Verbose: true, Stdout: &stdout, Stderr: &stderr}
	if err := runner.Run(context.Background(), project, "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "[hello] printf stdout; printf stderr >&2") {
		t.Fatalf("stdout = %q, want target progress", stdout.String())
	}
	if strings.Contains(stdout.String(), "] stdout") ||
		strings.Contains(stdout.String(), "] stderr") {
		t.Fatalf("stdout = %q, want command output suppressed", stdout.String())
	}
	if !strings.Contains(
		stdout.String(),
		"targets: success=1 cached=0 failed=0 preflight-failed=0 quality-failed=0 dry-run=0 running=0\n",
	) {
		t.Fatalf("stdout = %q, want summary", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestListRunsAppliesLimitNewestFirst(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"hello": shellTarget("hello", "printf ok"),
		},
	}

	for i := 0; i < 12; i++ {
		var out bytes.Buffer
		if err := (&Runner{LogOnly: true, Stdout: &out, Stderr: &out}).Run(
			context.Background(),
			project,
			"hello",
		); err != nil {
			t.Fatal(err)
		}
	}

	runs, err := newTestStore(t, project.StatePath).ListRuns(statestore.RunQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 10 {
		t.Fatalf("limited run count = %d, want 10", len(runs))
	}
	for index := 1; index < len(runs); index++ {
		if runs[index-1].StartedAt.Before(runs[index].StartedAt) {
			t.Fatalf("runs are not newest first: %#v", runs)
		}
	}

	allRuns, err := newTestStore(t, project.StatePath).ListRuns(statestore.RunQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(allRuns) != 12 {
		t.Fatalf("all run count = %d, want 12", len(allRuns))
	}
}
