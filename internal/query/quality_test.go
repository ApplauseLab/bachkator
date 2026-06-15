package query

import (
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/quality"
	statestore "github.com/applauselab/bachkator/internal/state"
)

type fakeQualityStore struct {
	reports  []quality.Report
	metrics  []quality.Metric
	findings []quality.Finding
	gates    []quality.GateResult
	runs     []statestore.RunRecord
}

func (s fakeQualityStore) ListQualityReports(int) ([]quality.Report, error) {
	return s.reports, nil
}

func (s fakeQualityStore) ListQualityMetrics(int) ([]quality.Metric, error) {
	return s.metrics, nil
}

func (s fakeQualityStore) ListQualityFindings(limit int) ([]quality.Finding, error) {
	if limit > 0 && len(s.findings) > limit {
		return s.findings[:limit], nil
	}
	return s.findings, nil
}
func (s fakeQualityStore) ListQualityGateResults(int) ([]quality.GateResult, error) {
	return s.gates, nil
}
func (s fakeQualityStore) ListRuns(statestore.RunQuery) ([]statestore.RunRecord, error) {
	return s.runs, nil
}

func TestQualityOrdersSlowTargetsAndFailingTests(t *testing.T) {
	now := time.Date(2026, 6, 13, 1, 0, 0, 0, time.UTC)
	snapshot, err := Quality(
		fakeQualityStore{
			findings: []quality.Finding{
				{Kind: "test-failure", Rule: "fast", DurationMS: 10},
				{Kind: "lint", Rule: "ignore", DurationMS: 1000},
				{Kind: "test-failure", Rule: "slow", DurationMS: 200},
			},
			runs: []statestore.RunRecord{{
				Targets: map[string]statestore.TargetRunRecord{
					"shell/fast": {
						StartedAt:  now,
						FinishedAt: now.Add(50 * time.Millisecond),
						Status:     "success",
					},
					"shell/slow": {
						StartedAt:  now,
						FinishedAt: now.Add(2 * time.Second),
						Status:     "failed",
					},
				},
			}},
		},
		QualityLimits{SlowTargets: 1, FailingTests: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.SlowTargets) != 1 || snapshot.SlowTargets[0].Name != "shell/slow" {
		t.Fatalf("slow targets = %#v", snapshot.SlowTargets)
	}
	if len(snapshot.FailingTests) != 1 || snapshot.FailingTests[0].Rule != "slow" {
		t.Fatalf("failing tests = %#v", snapshot.FailingTests)
	}
}
