package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordsRunCompletionAndListsRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store := NewStore(path)
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
	store := NewStore(filepath.Join(t.TempDir(), "state.db"))
	createdAt := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
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
