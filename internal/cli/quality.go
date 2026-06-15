package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
	"github.com/applauselab/bachkator/internal/query"
)

func runQuality(project *Project, deps Dependencies, args []string, stdout io.Writer) error {
	if deps.Quality == nil {
		return fmt.Errorf("quality query dependency is not configured")
	}
	mode := "summary"
	if len(args) > 0 {
		mode = args[0]
	}
	result := deps.Quality.QueryQuality(project, qualityLimitsForMode(mode))
	if result.Err != nil {
		return result.Err
	}
	snapshot := result.Snapshot
	switch mode {
	case "summary", "latest":
		return runQualitySummary(snapshot, stdout)
	case "reports":
		return printQualityReports(snapshot.Reports, stdout)
	case "metrics":
		return printQualityMetrics(snapshot.Metrics, stdout)
	case "findings":
		return printQualityFindings(snapshot.Findings, stdout)
	case "gates":
		return printQualityGates(snapshot.Gates, stdout)
	case "slow-targets":
		return printSlowTargets(snapshot.SlowTargets, stdout)
	case "failing-tests":
		return printFailingTests(snapshot.FailingTests, stdout)
	default:
		return fmt.Errorf(
			"unknown quality command %q; want summary, reports, metrics, findings, gates, slow-targets, or failing-tests",
			mode,
		)
	}
}

func qualityLimitsForMode(mode string) query.QualityLimits {
	limits := query.QualityLimits{}
	switch mode {
	case "summary", "latest":
		limits.Reports = 10
		limits.Gates = 10
		limits.SlowTargets = 10
		limits.FailingTests = 10
	case "reports":
		limits.Reports = 20
	case "metrics":
		limits.Metrics = 40
	case "findings":
		limits.Findings = 40
	case "gates":
		limits.Gates = 40
	case "slow-targets":
		limits.SlowTargets = 20
	case "failing-tests":
		limits.FailingTests = 20
	}
	return limits
}

func runQualitySummary(snapshot query.QualitySnapshot, stdout io.Writer) error {
	if err := printQualityReports(snapshot.Reports, stdout); err != nil {
		return err
	}
	if err := printQualityGates(snapshot.Gates, stdout); err != nil {
		return err
	}
	if err := printSlowTargets(snapshot.SlowTargets, stdout); err != nil {
		return err
	}
	return printFailingTests(snapshot.FailingTests, stdout)
}

func printQualityReports(reports []quality.Report, stdout io.Writer) error {
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

func printQualityMetrics(metrics []quality.Metric, stdout io.Writer) error {
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

func printQualityFindings(findings []quality.Finding, stdout io.Writer) error {
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

func printQualityGates(gates []quality.GateResult, stdout io.Writer) error {
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

func printSlowTargets(targets []query.SlowTarget, stdout io.Writer) error {
	if err := writeLine(stdout, "slow targets:"); err != nil {
		return err
	}
	for _, target := range targets {
		if err := writeLine(
			stdout,
			formatSlowTarget(target.Name, target.Duration, target.Status),
		); err != nil {
			return err
		}
	}
	return nil
}

func printFailingTests(tests []quality.Finding, stdout io.Writer) error {
	if err := writeLine(stdout, "top failing tests:"); err != nil {
		return err
	}
	for _, test := range tests {
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

func formatQualityReport(report quality.Report) string {
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

func formatQualityMetric(metric quality.Metric, scope string) string {
	return fmt.Sprintf("  %s scope=%s value=%.3f %s", metric.Name, scope, metric.Value, metric.Unit)
}

func formatQualityFinding(finding quality.Finding) string {
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

func formatQualityGateResult(gate quality.GateResult) string {
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

func formatSlowTarget(name string, duration time.Duration, status model.RunStatus) string {
	return fmt.Sprintf("  %s %s %s", name, duration.Round(time.Millisecond).String(), status)
}

func formatFailingTest(test quality.Finding) string {
	return fmt.Sprintf(
		"  %s duration=%s severity=%s message=%s",
		test.Rule,
		formatMilliseconds(test.DurationMS),
		test.Severity,
		test.Message,
	)
}
