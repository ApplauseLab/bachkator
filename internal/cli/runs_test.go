package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactsCommandListsIndexedArtifacts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "render" {
  shell   = "mkdir -p dist && printf manifest > \"$BACH_RUN_DIRECTORY/deploy.yaml\" && printf out > dist/app.txt"
  outputs = ["dist/app.txt"]
}
`)

	var runOut bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "run", "shell/render"}
	if err := Execute(context.Background(), args, &runOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	var artifactsOut bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "artifacts", "--target", "shell/render"}
	if err := Execute(
		context.Background(),
		args,
		&artifactsOut,
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}
	got := artifactsOut.String()
	for _, want := range []string{"shell/render", "manifest", "deploy.yaml", "artifact", "dist/app.txt", "log"} {
		if !strings.Contains(got, want) {
			t.Fatalf("artifacts output missing %q: %s", want, got)
		}
	}

	var runsOut bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "runs", "--artifact", "dist/app.txt"}
	if err := Execute(context.Background(), args, &runsOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runsOut.String(), "shell/render") ||
		!strings.Contains(runsOut.String(), "success") {
		t.Fatalf("runs output did not include artifact-matched run: %s", runsOut.String())
	}
}

func TestRunsInspectJSONIncludesQualityEvidenceAfterCommandFailure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "test" {
  shell = "mkdir -p .bach/artifacts && printf '%s\n' '<testsuite tests=\"1\" failures=\"1\" errors=\"0\" skipped=\"0\" time=\"0.01\"><testcase classname=\"Example\" name=\"fails\" time=\"0.01\"><failure message=\"nope\">failed</failure></testcase></testsuite>' > .bach/artifacts/junit.xml && exit 1"
  outputs = {
    junit = ".bach/artifacts/junit.xml"
  }
}

quality "shell.test" {
  junit {
    path   = shell.test.outputs.junit
    format = "junit-xml"
  }
}
`)

	var runOut bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "run", "shell/test"}
	if err := Execute(context.Background(), args, &runOut, &bytes.Buffer{}, "test"); err == nil {
		t.Fatal("run succeeded, want command failure")
	}
	runID := runIDFromSummary(runOut.String())
	if runID == "" {
		t.Fatalf("could not parse run id from output: %s", runOut.String())
	}

	var inspectOut bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "--json", "runs", "inspect", runID}
	if err := Execute(
		context.Background(),
		args,
		&inspectOut,
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}
	var inspection runInspection
	if err := json.Unmarshal(inspectOut.Bytes(), &inspection); err != nil {
		t.Fatalf("inspect JSON invalid: %v\n%s", err, inspectOut.String())
	}
	if inspection.Status != "failed" || len(inspection.FailedTargets) != 1 {
		t.Fatalf("inspection = %#v, want one failed run", inspection)
	}
	failure := inspection.FailedTargets[0]
	if failure.ExitCode == nil || *failure.ExitCode != 1 {
		t.Fatalf("exit code = %v, want 1", failure.ExitCode)
	}
	if len(failure.Quality.Reports) != 1 || !failure.Quality.Reports[0].Parsed {
		t.Fatalf("quality reports = %#v, want parsed junit", failure.Quality.Reports)
	}
	if failure.Quality.Reports[0].Findings != 2 {
		t.Fatalf("findings = %d, want 2", failure.Quality.Reports[0].Findings)
	}
}

func TestLogsCommandSlicesFailedTargetLog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "fail" {
  shell = "printf 'one\ntwo error\nthree\n' && exit 1"
}
`)

	var runOut bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "run", "shell/fail"}
	if err := Execute(context.Background(), args, &runOut, &bytes.Buffer{}, "test"); err == nil {
		t.Fatal("run succeeded, want command failure")
	}
	runID := runIDFromSummary(runOut.String())
	var logsOut bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "logs", runID, "--failed", "--last", "1"}
	if err := Execute(context.Background(), args, &logsOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	got := logsOut.String()
	if !strings.Contains(got, "three") || strings.Contains(got, "one") {
		t.Fatalf("logs output = %q, want last failed line only", got)
	}
}

func TestRunsInspectIncludesPreflightFix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "deploy" {
  shell = "true"
  preflights = [
    { name = "docker", kind = "session", command = ["sh", "-c", "exit 1"], fix = "Start Docker Desktop." },
  ]
}
`)

	var runOut bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "run", "shell/deploy"}
	if err := Execute(context.Background(), args, &runOut, &bytes.Buffer{}, "test"); err == nil {
		t.Fatal("run succeeded, want preflight failure")
	}
	runID := runIDFromSummary(runOut.String())
	var inspectOut bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "--json", "runs", "inspect", runID}
	if err := Execute(
		context.Background(),
		args,
		&inspectOut,
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspectOut.String(), "Start Docker Desktop.") {
		t.Fatalf("inspect output missing fix: %s", inspectOut.String())
	}
}

func runIDFromSummary(value string) string {
	for _, line := range strings.Split(value, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "run" {
			return fields[1]
		}
	}
	return ""
}
