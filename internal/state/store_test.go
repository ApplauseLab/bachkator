package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreRecordsRunCompletionAndListsRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	startedAt := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)

	run := RunRecord{
		ID:         "run-1",
		Target:     "shell/test",
		Status:     "success",
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Targets: map[string]TargetRunRecord{
			"shell/test": {
				Status:     "success",
				StartedAt:  startedAt,
				FinishedAt: finishedAt,
				LogPath:    ".bach/runs/run-1/shell-test.log",
				Operation:  "go test ./...",
			},
		},
		Artifacts: []ArtifactRecord{
			{
				RunID:     "run-1",
				Target:    "shell/test",
				Kind:      "log",
				Path:      ".bach/runs/run-1/shell-test.log",
				CreatedAt: finishedAt,
			},
		},
	}
	records := map[string]Record{
		"shell/test": {
			Fingerprint:      "abc",
			FingerprintParts: map[string]string{"files": "abc"},
			CompletedAt:      finishedAt,
		},
	}

	if err := store.RecordRunCompletion(records, run); err != nil {
		t.Fatal(err)
	}

	runs, err := store.ListRuns(
		RunQuery{Target: "shell/test", ArtifactPath: "shell-test", Limit: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != "run-1" {
		t.Fatalf("runs = %#v, want run-1", runs)
	}

	snapshot, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Targets["shell/test"].Fingerprint != "abc" {
		t.Fatalf("fingerprint = %q, want abc", snapshot.Targets["shell/test"].Fingerprint)
	}
}

func TestStoreSavesQualityReportsAndGates(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	createdAt := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	if err := store.RecordRunStart(RunRecord{
		ID:        "run-1",
		Target:    "shell/test",
		Status:    "running",
		StartedAt: createdAt,
		LogDir:    ".bach/runs/run-1",
		Targets:   map[string]TargetRunRecord{},
	}); err != nil {
		t.Fatal(err)
	}
	reports := []QualityReport{
		{
			RunID:     "run-1",
			Target:    "shell/test",
			Kind:      "coverage",
			Format:    "gocover",
			Path:      "coverage.out",
			Status:    "success",
			CreatedAt: createdAt,
			Metrics: []QualityMetric{
				{Name: "coverage.line.percent", Value: 91.5, Unit: "percent"},
			},
			Findings: []QualityFinding{
				{
					Kind:       "test-failure",
					Rule:       "TestExample",
					Severity:   "failure",
					Message:    "failed",
					DurationMS: 12,
				},
			},
		},
	}
	gates := []QualityGateResult{
		{
			RunID:     "run-1",
			Target:    "shell/test",
			Metric:    "coverage.line.percent",
			Op:        ">=",
			Threshold: 90,
			Actual:    91.5,
			Status:    "passed",
			CreatedAt: createdAt,
		},
	}

	if err := store.SaveQualityReports(reports, gates); err != nil {
		t.Fatal(err)
	}
	listedReports, err := store.ListQualityReports(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(listedReports) != 1 || listedReports[0].Path != "coverage.out" {
		t.Fatalf("reports = %#v, want coverage.out", listedReports)
	}
	listedGates, err := store.ListQualityGateResults(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(listedGates) != 1 || listedGates[0].Status != "passed" {
		t.Fatalf("gates = %#v, want passed gate", listedGates)
	}
}

func TestStoreRejectsQualityReportsForUnknownRun(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	err = store.SaveQualityReports(
		[]QualityReport{{
			RunID:     "missing-run",
			Target:    "shell/test",
			Kind:      "coverage",
			Format:    "go-cover",
			Path:      "coverage.out",
			Status:    "success",
			CreatedAt: time.Now().UTC(),
		}},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("SaveQualityReports() error = %v, want foreign key failure", err)
	}
}

func TestStoreRejectsSymlinkStatePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	realPath := filepath.Join(dir, "real.db")
	if err := os.WriteFile(realPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "state.db")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(linkPath)
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		if store != nil {
			_ = store.Close()
		}
		t.Fatalf("NewStore() error = %v, want symlink rejection", err)
	}
}

func TestStoreLoadRejectsSymlinkStatePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	realPath := filepath.Join(dir, "real.db")
	if err := os.WriteFile(realPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "state.db")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(realPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if _, err := store.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	roStore, err := OpenReadOnlyStore(linkPath)
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		if roStore != nil {
			_ = roStore.Close()
		}
		t.Fatalf("OpenReadOnlyStore() error = %v, want symlink rejection", err)
	}
}

func TestNewStoreEnablesForeignKeys(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	var enabled int
	if err := store.db.QueryRow(`PRAGMA foreign_keys;`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
}

func TestOpenReadOnlyStoreReturnsEmptyForMissingDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	store, err := OpenReadOnlyStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	snapshot, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Targets) != 0 || len(snapshot.Runs) != 0 {
		t.Fatalf("snapshot = %#v, want empty", snapshot)
	}
	runs, err := store.ListRuns(RunQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("runs = %#v, want empty", runs)
	}
}
