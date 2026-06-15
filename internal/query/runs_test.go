package query

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/quality"
	statestore "github.com/applauselab/bachkator/internal/state"
)

type fakeRunInspectStore struct {
	state   *statestore.State
	reports []quality.Report
	gates   []quality.GateResult
}

func (s fakeRunInspectStore) Load() (*statestore.State, error) { return s.state, nil }
func (s fakeRunInspectStore) QualityReportsForRun(string) ([]quality.Report, error) {
	return s.reports, nil
}
func (s fakeRunInspectStore) QualityGateResultsForRun(string) ([]quality.GateResult, error) {
	return s.gates, nil
}

func TestInspectRunAggregatesQualityAndArtifacts(t *testing.T) {
	now := time.Date(2026, 6, 13, 1, 0, 0, 0, time.UTC)
	exitCode := 2
	inspection, err := InspectRun(
		fakeRunInspectStore{
			state: &statestore.State{Runs: []statestore.RunRecord{{
				ID:        "run-1",
				Target:    "shell/test",
				Status:    "failed",
				StartedAt: now,
				LogDir:    ".bach/runs/run-1",
				Targets: map[string]statestore.TargetRunRecord{
					"shell/test": {
						Status:    "failed",
						Operation: "go test ./...",
						LogPath:   ".bach/runs/run-1/shell-test.log",
						ExitCode:  &exitCode,
					},
				},
				Artifacts: []statestore.ArtifactRecord{{
					Target: "shell/test",
					Kind:   "log",
					Path:   ".bach/runs/run-1/shell-test.log",
				}},
			}}},
			reports: []quality.Report{{
				RunID:    "run-1",
				Target:   "shell/test",
				Kind:     "junit",
				Format:   "junit-xml",
				Path:     "junit.xml",
				Status:   "success",
				Findings: []quality.Finding{{Kind: "test-failure"}},
			}},
			gates: []quality.GateResult{{
				RunID:   "run-1",
				Target:  "shell/test",
				Metric:  "coverage.line.percent",
				Op:      ">=",
				Status:  "failed",
				Message: "coverage too low",
			}},
		},
		RunInspectOptions{RunID: "run-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.FailedTargets) != 1 || inspection.FailedTargets[0].ExitCode == nil ||
		*inspection.FailedTargets[0].ExitCode != exitCode {
		t.Fatalf("failed targets = %#v", inspection.FailedTargets)
	}
	quality := inspection.FailedTargets[0].Quality
	if len(quality.Reports) != 1 || quality.Reports[0].Findings != 1 || !quality.Reports[0].Parsed {
		t.Fatalf("quality reports = %#v", quality.Reports)
	}
	if len(quality.FailedGates) != 1 || quality.FailedGates[0].Message != "coverage too low" {
		t.Fatalf("failed gates = %#v", quality.FailedGates)
	}
	if len(inspection.FailedTargets[0].Artifacts) != 1 {
		t.Fatalf("artifacts = %#v", inspection.FailedTargets[0].Artifacts)
	}
}

func TestLogsFiltersFailedTargetsAndErrorLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".bach", "runs", "run-1", "shell-test.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("one\ntwo error\nthree failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sections, err := Logs(
		fakeRunInspectStore{state: &statestore.State{Runs: []statestore.RunRecord{{
			ID:     "run-1",
			Target: "shell/test",
			Targets: map[string]statestore.TargetRunRecord{
				"shell/ok": {
					Status:  "success",
					LogPath: filepath.Join(".bach", "runs", "run-1", "ok.log"),
				},
				"shell/test": {Status: "failed", LogPath: logPath},
			},
		}}}},
		LogOptions{RunID: "run-1", Root: dir, FailedOnly: true, Last: 1, ErrorsOnly: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Target != "shell/test" {
		t.Fatalf("sections = %#v", sections)
	}
	if len(sections[0].Lines) != 1 || sections[0].Lines[0] != "three failed" {
		t.Fatalf("lines = %#v", sections[0].Lines)
	}
}
