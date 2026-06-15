package query

import (
	"sort"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
	statestore "github.com/applauselab/bachkator/internal/state"
)

type QualityLimits struct {
	Reports      int
	Metrics      int
	Findings     int
	Gates        int
	SlowTargets  int
	FailingTests int
}

type QualitySnapshot struct {
	Reports      []quality.Report
	Metrics      []quality.Metric
	Findings     []quality.Finding
	Gates        []quality.GateResult
	SlowTargets  []SlowTarget
	FailingTests []quality.Finding
}

type SlowTarget struct {
	Name     string
	Duration time.Duration
	Status   model.RunStatus
}

type QualityStore interface {
	ListQualityReports(limit int) ([]quality.Report, error)
	ListQualityMetrics(limit int) ([]quality.Metric, error)
	ListQualityFindings(limit int) ([]quality.Finding, error)
	ListQualityGateResults(limit int) ([]quality.GateResult, error)
	ListRuns(statestore.RunQuery) ([]statestore.RunRecord, error)
}

func Quality(store QualityStore, limits QualityLimits) (QualitySnapshot, error) {
	reports, err := store.ListQualityReports(limits.Reports)
	if err != nil {
		return QualitySnapshot{}, err
	}
	metrics, err := store.ListQualityMetrics(limits.Metrics)
	if err != nil {
		return QualitySnapshot{}, err
	}
	findings, err := store.ListQualityFindings(limits.Findings)
	if err != nil {
		return QualitySnapshot{}, err
	}
	gates, err := store.ListQualityGateResults(limits.Gates)
	if err != nil {
		return QualitySnapshot{}, err
	}
	runs, err := store.ListRuns(statestore.RunQuery{})
	if err != nil {
		return QualitySnapshot{}, err
	}
	allFindings, err := store.ListQualityFindings(0)
	if err != nil {
		return QualitySnapshot{}, err
	}
	return QualitySnapshot{
		Reports:      reports,
		Metrics:      metrics,
		Findings:     findings,
		Gates:        gates,
		SlowTargets:  slowTargets(runs, limits.SlowTargets),
		FailingTests: failingTests(allFindings, limits.FailingTests),
	}, nil
}

func slowTargets(runs []statestore.RunRecord, limit int) []SlowTarget {
	targets := []SlowTarget{}
	for _, run := range runs {
		for name, target := range run.Targets {
			if target.FinishedAt.IsZero() || target.StartedAt.IsZero() {
				continue
			}
			duration := target.FinishedAt.Sub(target.StartedAt)
			if duration < 0 {
				continue
			}
			targets = append(targets, SlowTarget{
				Name:     name,
				Duration: duration,
				Status:   target.Status,
			})
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Duration > targets[j].Duration })
	if limit > 0 && len(targets) > limit {
		return targets[:limit]
	}
	return targets
}

func failingTests(findings []quality.Finding, limit int) []quality.Finding {
	tests := []quality.Finding{}
	for _, finding := range findings {
		if finding.Kind == "test-failure" {
			tests = append(tests, finding)
		}
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].DurationMS == tests[j].DurationMS {
			return strings.Compare(tests[i].Rule, tests[j].Rule) < 0
		}
		return tests[i].DurationMS > tests[j].DurationMS
	})
	if limit > 0 && len(tests) > limit {
		return tests[:limit]
	}
	return tests
}
