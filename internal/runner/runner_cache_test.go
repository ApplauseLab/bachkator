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

func TestRunnerUsesStateForIncrementalBuilds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"build": shellTarget(
				"build",
				"mkdir -p out && date +%s%N >> out/runs.txt",
				withInputs("input.txt"),
				withOutputs("out/runs.txt"),
			),
		},
	}

	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "build"); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(context.Background(), project, "build"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "out", "runs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(contents), "\n"); lines != 1 {
		t.Fatalf("run count after cached run = %d, want 1", lines)
	}

	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(context.Background(), project, "build"); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(filepath.Join(dir, "out", "runs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(contents), "\n"); lines != 2 {
		t.Fatalf("run count after input change = %d, want 2", lines)
	}
}

func TestRunnerDryRunDoesNotCreateStateDB(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"build": shellTarget("build", "printf nope", withInputs("input.txt")),
		},
	}
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(project.StatePath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created state db, stat error = %v", err)
	}
}

func TestRunnerDryRunDoesNotPersistRunsArtifactsOrCacheState(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/render": shellTarget(
				"shell/render",
				`mkdir -p dist && printf out > dist/app.txt`,
				withInputs("input.txt"),
				withOutputs("dist/app.txt"),
			),
			"image/app": imageTarget("image/app", "example/app", []string{"dev"}),
		},
	}
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/render",
	); err != nil {
		t.Fatal(err)
	}
	before, err := newTestStore(t, project.StatePath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Runs) != 1 || before.Targets["shell/render"].Fingerprint == "" {
		t.Fatalf("state before dry-run = %#v", before)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"image/app",
	); err != nil {
		t.Fatal(err)
	}
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/render",
	); err != nil {
		t.Fatal(err)
	}
	after, err := newTestStore(t, project.StatePath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Runs) != 1 || after.Runs[0].DryRun {
		t.Fatalf("runs after dry-run = %#v", after.Runs)
	}
	if after.Targets["shell/render"].Fingerprint != before.Targets["shell/render"].Fingerprint {
		t.Fatalf(
			"dry-run updated target cache state: before=%#v after=%#v",
			before.Targets,
			after.Targets,
		)
	}
	artifacts, err := newTestStore(t, project.StatePath).
		ListArtifacts(statestore.ArtifactQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		if artifact.Target == "image/app" {
			t.Fatalf("dry-run persisted image artifact: %#v", artifacts)
		}
	}
}

func TestProfileEnvInvalidatesFingerprint(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		Root:             dir,
		StatePath:        filepath.Join(dir, ".bach", "state.db"),
		SelectedProfiles: []string{"staging"},
		ProfileEnv:       []string{"APP_ENV=staging"},
		Targets: map[string]*Target{
			"build": shellTarget(
				"build",
				"mkdir -p out && printf '%s\n' \"$APP_ENV\" >> out/runs.txt",
				withInputs("input.txt"),
				withOutputs("out/runs.txt"),
			),
		},
	}

	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "build"); err != nil {
		t.Fatal(err)
	}
	project.ProfileEnv = []string{"APP_ENV=staging-kristiyan"}
	if err := runner.Run(context.Background(), project, "build"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "out", "runs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(contents), "staging\nstaging-kristiyan\n"; got != want {
		t.Fatalf("runs = %q, want %q", got, want)
	}
}

func TestRunnerForceIgnoresFreshState(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"build": shellTarget(
				"build",
				"mkdir -p out && date +%s%N >> out/runs.txt",
				withInputs("input.txt"),
				withOutputs("out/runs.txt"),
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
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
	if err := (&Runner{Force: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(dir, "out", "runs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(contents), "\n"); lines != 2 {
		t.Fatalf("run count after forced run = %d, want 2", lines)
	}
}

func TestStaleReasonsIdentifyCacheInvalidationCause(t *testing.T) {
	dir := t.TempDir()
	target := shellTarget("build", "", withOutputs("out/app"))
	record := StateRecord{
		Fingerprint: "old",
		FingerprintParts: map[string]string{
			"inputs":       "old-inputs",
			"env":          "old-env",
			"operation":    "old-operation",
			"dependencies": "old-dependencies",
		},
	}
	parts := map[string]string{
		"inputs":       "new-inputs",
		"env":          "new-env",
		"operation":    "new-operation",
		"dependencies": "new-dependencies",
	}

	reasons := targetStaleReasons(target, dir, record, "new", parts, true)
	for _, want := range []string{"forced run", "changed input", "changed env var", "changed operation", "dependency fingerprint change", "missing output"} {
		if !containsString(reasons, want) {
			t.Fatalf("stale reasons = %#v, want %q", reasons, want)
		}
	}
}

func TestDryRunWithInputsDoesNotPersistState(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"build": shellTarget(
				"build",
				"printf nope",
				withInputs("missing-is-ok-for-dry-run.txt"),
			),
		},
	}
	if err := os.WriteFile(
		filepath.Join(dir, "missing-is-ok-for-dry-run.txt"),
		[]byte("x"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(project.StatePath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created state db, stat error = %v", err)
	}
}

func TestDryRunFreshTargetReportsDryRunStatus(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"build": shellTarget(
				"build",
				"mkdir -p out && cp input.txt out/app.txt",
				withInputs("input.txt"),
				withOutputs("out/app.txt"),
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
	out.Reset()
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "[build] (cached) mkdir -p out && cp input.txt out/app.txt") {
		t.Fatalf("stdout = %q, want cached operation", got)
	}
	if !strings.Contains(
		got,
		"targets: success=0 cached=0 failed=0 preflight-failed=0 quality-failed=0 dry-run=1 running=0",
	) {
		t.Fatalf("stdout = %q, want dry-run target count", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
