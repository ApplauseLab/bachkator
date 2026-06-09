package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	statestore "github.com/applause/bachkator/internal/state"
)

func runQuality(project *Project, args []string, stdout io.Writer) error {
	mode := "summary"
	if len(args) > 0 {
		mode = args[0]
	}
	switch mode {
	case "summary", "latest":
		return runQualitySummary(project, stdout)
	case "reports":
		return printQualityReports(project, stdout, 20)
	case "metrics":
		return printQualityMetrics(project, stdout, 40)
	case "findings":
		return printQualityFindings(project, stdout, 40)
	case "gates":
		return printQualityGates(project, stdout, 40)
	case "slow-targets":
		return printSlowTargets(project, stdout, 20)
	case "failing-tests":
		return printFailingTests(project, stdout, 20)
	default:
		return fmt.Errorf(
			"unknown quality command %q; want summary, reports, metrics, findings, gates, slow-targets, or failing-tests",
			mode,
		)
	}
}

func runQualitySummary(project *Project, stdout io.Writer) error {
	if err := printQualityReports(project, stdout, 10); err != nil {
		return err
	}
	if err := printQualityGates(project, stdout, 10); err != nil {
		return err
	}
	if err := printSlowTargets(project, stdout, 10); err != nil {
		return err
	}
	return printFailingTests(project, stdout, 10)
}

func printQualityReports(project *Project, stdout io.Writer, limit int) error {
	reports, err := statestore.NewStore(project.StatePath).ListQualityReports(limit)
	if err != nil {
		return err
	}
	if err := writeLine(stdout, "quality reports:"); err != nil {
		return err
	}
	for _, report := range reports {
		if err := writeLine(stdout, formatQualityReport(report)); err != nil {
			return err
		}
	}
	return nil
}

func printQualityMetrics(project *Project, stdout io.Writer, limit int) error {
	metrics, err := statestore.NewStore(project.StatePath).ListQualityMetrics(limit)
	if err != nil {
		return err
	}
	if err := writeLine(stdout, "quality metrics:"); err != nil {
		return err
	}
	for _, metric := range metrics {
		scope := metric.Scope
		if scope == "" {
			scope = "global"
		}
		if err := writeLine(stdout, formatQualityMetric(metric, scope)); err != nil {
			return err
		}
	}
	return nil
}

func printQualityFindings(project *Project, stdout io.Writer, limit int) error {
	findings, err := statestore.NewStore(project.StatePath).ListQualityFindings(limit)
	if err != nil {
		return err
	}
	if err := writeLine(stdout, "quality findings:"); err != nil {
		return err
	}
	for _, finding := range findings {
		if err := writeLine(stdout, formatQualityFinding(finding)); err != nil {
			return err
		}
	}
	return nil
}

func printQualityGates(project *Project, stdout io.Writer, limit int) error {
	gates, err := statestore.NewStore(project.StatePath).ListQualityGateResults(limit)
	if err != nil {
		return err
	}
	if err := writeLine(stdout, "quality gates:"); err != nil {
		return err
	}
	for _, gate := range gates {
		if err := writeLine(stdout, formatQualityGateResult(gate)); err != nil {
			return err
		}
	}
	return nil
}

func printSlowTargets(project *Project, stdout io.Writer, limit int) error {
	runs, err := statestore.NewStore(project.StatePath).ListRuns(statestore.RunQuery{})
	if err != nil {
		return err
	}
	type targetDuration struct {
		Name     string
		Duration time.Duration
		Status   string
	}
	var targets []targetDuration
	for _, run := range runs {
		for name, target := range run.Targets {
			if target.FinishedAt.IsZero() || target.StartedAt.IsZero() {
				continue
			}
			duration := target.FinishedAt.Sub(target.StartedAt)
			if duration < 0 {
				continue
			}
			targets = append(
				targets,
				targetDuration{Name: name, Duration: duration, Status: target.Status},
			)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Duration > targets[j].Duration })
	if err := writeLine(stdout, "slow targets:"); err != nil {
		return err
	}
	for index, target := range targets {
		if limit > 0 && index >= limit {
			break
		}
		if err := writeLine(
			stdout,
			formatSlowTarget(target.Name, target.Duration, target.Status),
		); err != nil {
			return err
		}
	}
	return nil
}

func printFailingTests(project *Project, stdout io.Writer, limit int) error {
	findings, err := statestore.NewStore(project.StatePath).ListQualityFindings(0)
	if err != nil {
		return err
	}
	tests := make([]statestore.QualityFinding, 0)
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
	if err := writeLine(stdout, "top failing tests:"); err != nil {
		return err
	}
	for index, test := range tests {
		if limit > 0 && index >= limit {
			break
		}
		if err := writeLine(stdout, formatFailingTest(test)); err != nil {
			return err
		}
	}
	return nil
}

func formatMilliseconds(ms float64) string {
	return time.Duration(ms * float64(time.Millisecond)).Round(time.Millisecond).String()
}

func writeLine(w io.Writer, value string) error {
	_, err := fmt.Fprintln(w, value)
	return err
}

func formatQualityReport(report statestore.QualityReport) string {
	return fmt.Sprintf(
		"  %s %s %s format=%s status=%s path=%s",
		report.RunID,
		report.Target,
		report.Kind,
		report.Format,
		report.Status,
		report.Path,
	)
}

func formatQualityMetric(metric statestore.QualityMetric, scope string) string {
	return fmt.Sprintf("  %s scope=%s value=%.3f %s", metric.Name, scope, metric.Value, metric.Unit)
}

func formatQualityFinding(finding statestore.QualityFinding) string {
	return fmt.Sprintf(
		"  %s %s severity=%s file=%s line=%d duration=%s message=%s",
		finding.Kind,
		finding.Rule,
		finding.Severity,
		finding.File,
		finding.Line,
		formatMilliseconds(finding.DurationMS),
		finding.Message,
	)
}

func formatQualityGateResult(gate statestore.QualityGateResult) string {
	return fmt.Sprintf(
		"  %s %s %s %s %.3f actual=%.3f status=%s",
		gate.RunID,
		gate.Target,
		gate.Metric,
		gate.Op,
		gate.Threshold,
		gate.Actual,
		gate.Status,
	)
}

func formatSlowTarget(name string, duration time.Duration, status string) string {
	return fmt.Sprintf("  %s duration=%s status=%s", name, duration.Round(time.Millisecond), status)
}

func formatFailingTest(test statestore.QualityFinding) string {
	return fmt.Sprintf(
		"  %s duration=%s severity=%s message=%s",
		test.Rule,
		formatMilliseconds(test.DurationMS),
		test.Severity,
		test.Message,
	)
}
