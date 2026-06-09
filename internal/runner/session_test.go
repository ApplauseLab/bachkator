package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionCompletesDryRunWithoutPersistingState(t *testing.T) {
	dir := t.TempDir()
	project := sessionProject(t, dir)
	state := &State{Targets: map[string]StateRecord{}}
	plan, err := BuildPlan(project, "build")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	run := newRunRecord(project, "build", true, false)
	s := newSession(
		Runner{DryRun: true, Stdout: &out, Stderr: &out},
		project,
		state,
		&run,
		plan,
		GitContext{},
		nil,
	)

	if err := s.completeRun("success"); err != nil {
		t.Fatal(err)
	}
	if run.Status != "success" || run.FinishedAt.IsZero() {
		t.Fatalf("run = %#v", run)
	}
	if !strings.Contains(out.String(), "run ") ||
		!strings.Contains(out.String(), " success target=build ") {
		t.Fatalf("summary = %q", out.String())
	}
	if _, err := os.Stat(project.StatePath); !os.IsNotExist(err) {
		t.Fatalf("dry-run persisted state, stat error = %v", err)
	}
}

func TestSessionTargetLifecycle(t *testing.T) {
	dir := t.TempDir()
	project := sessionProject(t, dir)
	state := &State{Targets: map[string]StateRecord{}}
	plan, err := BuildPlan(project, "build")
	if err != nil {
		t.Fatal(err)
	}
	run := newRunRecord(project, "build", true, false)
	s := newSession(
		Runner{DryRun: true, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}},
		project,
		state,
		&run,
		plan,
		GitContext{},
		nil,
	)

	record := s.startTarget(project.Targets["build"], "printf ok")
	if record.Status != "running" || record.Operation != "printf ok" || record.LogPath == "" {
		t.Fatalf("started record = %#v", record)
	}
	s.finishTarget("build", "success")
	finished := run.Targets["build"]
	if finished.Status != "success" || finished.FinishedAt.IsZero() || finished.StartedAt.IsZero() {
		t.Fatalf("finished record = %#v", finished)
	}
}

func TestSessionRecordsSyntheticPreflightFailureLog(t *testing.T) {
	dir := t.TempDir()
	project := sessionProject(t, dir)
	state := &State{Targets: map[string]StateRecord{}}
	plan, err := BuildPlan(project, "build")
	if err != nil {
		t.Fatal(err)
	}
	run := newRunRecord(project, "build", false, false)
	s := newSession(
		Runner{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}},
		project,
		state,
		&run,
		plan,
		GitContext{},
		nil,
	)

	s.recordPreflightFailure(
		PreflightFailure{
			Preflight: PreflightCheck{Name: "cloud"},
			Targets:   []string{"build"},
			Reason:    "expired",
		},
		"credential/session preflights failed",
	)
	record := run.Targets["build"]
	if record.Status != "preflight-failed" || record.Operation != "credential/session preflight" ||
		record.LogPath == "" {
		t.Fatalf("record = %#v", record)
	}
	contents, err := os.ReadFile(record.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "credential/session preflight failed") {
		t.Fatalf("log = %q", string(contents))
	}
}

func TestSessionFingerprintState(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"dep":   shellTarget("dep", "printf dep"),
			"build": shellTarget("build", "printf build", withDependsOn("dep")),
		},
	}
	state := &State{Targets: map[string]StateRecord{}}
	plan, err := BuildPlan(project, "build")
	if err != nil {
		t.Fatal(err)
	}
	run := newRunRecord(project, "build", true, false)
	s := newSession(
		Runner{DryRun: true, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}},
		project,
		state,
		&run,
		plan,
		GitContext{},
		nil,
	)

	s.rememberFingerprint("dep", "abc")
	inputs := s.dependencyFingerprints(plan, "build")
	if inputs["dep"] != "abc" {
		t.Fatalf("inputs = %#v", inputs)
	}
	record := StateRecord{Fingerprint: "new"}
	s.markTargetDirty("build", record)
	if s.dirtyTargets["build"].Fingerprint != "new" || state.Targets["build"].Fingerprint != "new" {
		t.Fatalf("dirty=%#v state=%#v", s.dirtyTargets, state.Targets)
	}
}

func sessionProject(t *testing.T, dir string) *Project {
	t.Helper()
	return &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"build": shellTarget("build", "printf ok", withOutputs("out.txt")),
		},
	}
}
