package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
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
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "flaky")
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
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
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
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "quality")
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
	if err := (&Runner{Stdout: &out, Stderr: &out}).Run(
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
	gates, err := newTestStore(t, project.StatePath).ListQualityGateResults(10)
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
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "quality")
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
