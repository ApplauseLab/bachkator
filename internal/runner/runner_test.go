package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	statestore "github.com/applause/bachkator/internal/state"
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
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"ok",
	); err != nil {
		t.Fatal(err)
	}
}

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
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/render",
	); err != nil {
		t.Fatal(err)
	}
	artifacts, err := statestore.NewStore(project.StatePath).
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
	if err := (Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/render",
	); err != nil {
		t.Fatal(err)
	}
	before, err := statestore.NewStore(project.StatePath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Runs) != 1 || before.Targets["shell/render"].Fingerprint == "" {
		t.Fatalf("state before dry-run = %#v", before)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := (Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"image/app",
	); err != nil {
		t.Fatal(err)
	}
	if err := (Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/render",
	); err != nil {
		t.Fatal(err)
	}
	after, err := statestore.NewStore(project.StatePath).Load()
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
	artifacts, err := statestore.NewStore(project.StatePath).
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
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "shell/deploy")
	if err == nil ||
		!strings.Contains(err.Error(), `target "shell/deploy" requires confirmation`) ||
		!strings.Contains(err.Error(), "-dry-run") ||
		!strings.Contains(err.Error(), "-yes") {
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
	if err := (Runner{Yes: true, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "shell/deploy")
	if err == nil ||
		!strings.Contains(err.Error(), `target "shell/deploy" requires confirmation`) ||
		!strings.Contains(err.Error(), "destructive") {
		t.Fatalf("error = %v, want inherited confirmation guard", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "applied.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("guarded dependency wrote output, stat error = %v", statErr)
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
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "release")
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
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "deploy")
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
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "slow")
	if err == nil || !strings.Contains(err.Error(), `target "slow" timed out after 20ms`) {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestRunnerRetriesCommandUntilSuccess(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "flaky.sh", `n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf "%s" "$n" > attempts
test "$n" -ge 3
`)
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"flaky": commandTarget(
				"flaky",
				[]string{"sh", "flaky.sh"},
				withRetry(RetryPolicy{Attempts: 3}),
			),
		},
	}

	var out bytes.Buffer
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"flaky",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "3" {
		t.Fatalf("attempt count = %q, want 3", string(contents))
	}
}

func TestRunnerReturnsRetryExhaustionError(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "flaky.sh", `n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf "%s" "$n" > attempts
false
`)
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"flaky": commandTarget(
				"flaky",
				[]string{"sh", "flaky.sh"},
				withRetry(RetryPolicy{Attempts: 2}),
			),
		},
	}

	var out bytes.Buffer
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "flaky")
	if err == nil {
		t.Fatal("expected retry exhaustion error")
	}
	contents, readErr := os.ReadFile(filepath.Join(dir, "attempts"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != "2" {
		t.Fatalf("attempt count = %q, want 2", string(contents))
	}
}

func TestRunnerRetriesCompletionContractFailures(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "flaky-contract.sh", `n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf "%s" "$n" > attempts
`)
	writeScript(t, dir, "contract-check.sh", `test $(cat attempts) -ge 2
`)
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"flaky-contract": commandTarget(
				"flaky-contract",
				[]string{"sh", "flaky-contract.sh"},
				withRetry(RetryPolicy{Attempts: 2}),
				withSuccess(CompletionCheck{Command: []string{"sh", "contract-check.sh"}}),
			),
		},
	}

	var out bytes.Buffer
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"flaky-contract",
	); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "2" {
		t.Fatalf("attempt count = %q, want 2", string(contents))
	}
}

func TestRunnerDoesNotRetryQualityFailures(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "quality.sh", `n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf "%s" "$n" > attempts
`)
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"quality": commandTarget(
				"quality",
				[]string{"sh", "quality.sh"},
				withRetry(RetryPolicy{Attempts: 3}),
				withQualityGate(QualityGateSpec{Metric: "tests.failed", Max: ptrFloat64(0)}),
			),
		},
	}

	var out bytes.Buffer
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "quality")
	if err == nil || !strings.Contains(err.Error(), "quality gates failed") {
		t.Fatalf("error = %v, want quality failure", err)
	}
	contents, readErr := os.ReadFile(filepath.Join(dir, "attempts"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != "1" {
		t.Fatalf("attempt count = %q, want 1", string(contents))
	}
}

func TestRunnerRetriesQualityGateFailuresWhenOptedIn(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "quality.sh", `n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf "%s" "$n" > attempts
if [ "$n" -lt 2 ]; then
  printf '<testsuite tests="1" failures="1" errors="0" skipped="0"><testcase name="bad"><failure message="not yet"/></testcase></testsuite>' > junit.xml
else
  printf '<testsuite tests="1" failures="0" errors="0" skipped="0"><testcase name="ok"/></testsuite>' > junit.xml
fi
`)
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"quality": commandTarget(
				"quality",
				[]string{"sh", "quality.sh"},
				withRetry(RetryPolicy{Attempts: 3, RetryOnQualityGateFailure: true}),
				withQualityReport(QualityReportDeclaration{
					Kind:   "test",
					Format: "junit-xml",
					Path:   "junit.xml",
				}),
				withQualityGate(QualityGateSpec{Metric: "tests.failed", Max: ptrFloat64(0)}),
			),
		},
	}

	var out bytes.Buffer
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"quality",
	); err != nil {
		t.Fatal(err)
	}
	contents, readErr := os.ReadFile(filepath.Join(dir, "attempts"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != "2" {
		t.Fatalf("attempt count = %q, want 2", string(contents))
	}
	gates, err := statestore.NewStore(project.StatePath).ListQualityGateResults(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 1 || gates[0].Status != "success" {
		t.Fatalf("gates = %#v, want only final successful gate", gates)
	}
}

func TestRunnerDoesNotRetryQualityParseFailures(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "quality.sh", `n=$(cat attempts 2>/dev/null || printf 0)
n=$((n + 1))
printf "%s" "$n" > attempts
printf 'not xml' > junit.xml
`)
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"quality": commandTarget(
				"quality",
				[]string{"sh", "quality.sh"},
				withRetry(RetryPolicy{Attempts: 3, RetryOnQualityGateFailure: true}),
				withQualityReport(QualityReportDeclaration{
					Kind:   "test",
					Format: "junit-xml",
					Path:   "junit.xml",
				}),
			),
		},
	}

	var out bytes.Buffer
	err := (Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "quality")
	if err == nil || !strings.Contains(err.Error(), "quality reports failed") {
		t.Fatalf("error = %v, want quality parse failure", err)
	}
	contents, readErr := os.ReadFile(filepath.Join(dir, "attempts"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(contents) != "1" {
		t.Fatalf("attempt count = %q, want 1", string(contents))
	}
}

func ptrFloat64(value float64) *float64 { return &value }

func writeScript(t *testing.T, dir string, name string, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
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
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
	if err := (Runner{Force: true, Stdout: &out, Stderr: &out}).Run(
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

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
	if err := (Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{Parallelism: 3, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{Parallelism: 3, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/outer",
	)
	if err == nil || !strings.Contains(err.Error(), `target "pipeline/inner" timed out`) {
		t.Fatalf("error = %v, want inner pipeline timeout", err)
	}
}

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
	if err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"pipeline/deploy",
	)
	if err == nil {
		t.Fatal("expected pipeline failure")
	}
	runs, err := statestore.NewStore(project.StatePath).ListRuns(statestore.RunQuery{})
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
	if err := (Runner{Parallelism: 3, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{DryRun: true, Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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
	err := (Runner{Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{Parallelism: 2, Stdout: &out, Stderr: &out}).Run(
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

func TestRunnerInjectsGitContextEnvironment(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	runGit(t, dir, "init")
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "staged.txt")
	if err := os.WriteFile(
		filepath.Join(dir, "unstaged.txt"),
		[]byte("unstaged"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"git-context": shellTarget(
				"git-context",
				"printf '%s' \"$BACH_GIT_STAGED_FILES\" > staged-env.txt; printf '%s' \"$BACH_GIT_UNTRACKED_FILES\" > untracked-env.txt; printf '%s' \"$BACH_GIT_DIRTY\" > dirty-env.txt",
			),
		},
	}

	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "git-context"); err != nil {
		t.Fatal(err)
	}
	staged, err := os.ReadFile(filepath.Join(dir, "staged-env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(staged) != "staged.txt" {
		t.Fatalf("staged files env = %q, want staged.txt", string(staged))
	}
	untracked, err := os.ReadFile(filepath.Join(dir, "untracked-env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(untracked), "unstaged.txt") {
		t.Fatalf("untracked files env = %q, want unstaged.txt", string(untracked))
	}
	dirty, err := os.ReadFile(filepath.Join(dir, "dirty-env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(dirty) != "1" {
		t.Fatalf("dirty env = %q, want 1", string(dirty))
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

	runs, err := statestore.NewStore(project.StatePath).ListRuns(statestore.RunQuery{})
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

	runs, err := statestore.NewStore(project.StatePath).ListRuns(statestore.RunQuery{})
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

	runs, err := statestore.NewStore(project.StatePath).ListRuns(statestore.RunQuery{})
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
		if err := (Runner{LogOnly: true, Stdout: &out, Stderr: &out}).Run(
			context.Background(),
			project,
			"hello",
		); err != nil {
			t.Fatal(err)
		}
	}

	runs, err := statestore.NewStore(project.StatePath).ListRuns(statestore.RunQuery{Limit: 10})
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

	allRuns, err := statestore.NewStore(project.StatePath).ListRuns(statestore.RunQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(allRuns) != 12 {
		t.Fatalf("all run count = %d, want 12", len(allRuns))
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
	if err := (Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
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
	if err := (Runner{Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"build",
	); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := (Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %s\n%s", strings.Join(args, " "), err, string(output))
	}
}
