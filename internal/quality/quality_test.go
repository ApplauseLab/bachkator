package quality

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/state"
)

func TestParseJUnitXMLReportsDurationsAndFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(
		path,
		[]byte(
			`<testsuite tests="2" failures="1" errors="0" skipped="0" time="1.25"><testcase classname="pkg" name="fast" time="0.25"/><testcase classname="pkg" name="slow" time="1.0"><failure message="boom"/></testcase></testsuite>`,
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	report, err := parseJUnitXML(path)
	if err != nil {
		t.Fatal(err)
	}
	if metricValue(report.Metrics, "tests.total") != 2 ||
		metricValue(report.Metrics, "tests.failed") != 1 ||
		metricValue(report.Metrics, "tests.duration.ms") != 1250 {
		t.Fatalf("metrics = %#v", report.Metrics)
	}
	if len(report.Findings) != 3 {
		t.Fatalf("findings = %#v, want tests plus failure", report.Findings)
	}
}

func TestParseGoCoverReportsCoveragePercent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coverage.out")
	if err := os.WriteFile(
		path,
		[]byte("mode: set\nexample.go:1.1,2.1 2 1\nexample.go:3.1,4.1 2 0\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	report, err := parseGoCover(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := metricValue(report.Metrics, "coverage.line.percent"); got != 50 {
		t.Fatalf("coverage = %v, want 50", got)
	}
}

func TestParseCoberturaXMLReportsCoveragePercent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cobertura.xml")
	if err := os.WriteFile(
		path,
		[]byte(
			`<coverage line-rate="0.82" branch-rate="0.50" lines-covered="82" lines-valid="100"></coverage>`,
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	report, err := parseCoberturaXML(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := metricValue(report.Metrics, "coverage.line.percent"); got != 82 {
		t.Fatalf("coverage.line.percent = %v, want 82", got)
	}
}

func TestParseJaCoCoXMLReportsCoveragePercent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jacoco.xml")
	if err := os.WriteFile(
		path,
		[]byte(
			`<report><counter type="LINE" missed="20" covered="80"/><counter type="BRANCH" missed="5" covered="5"/><counter type="COMPLEXITY" missed="2" covered="8"/></report>`,
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	report, err := parseJaCoCoXML(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := metricValue(report.Metrics, "coverage.line.percent"); got != 80 {
		t.Fatalf("coverage.line.percent = %v, want 80", got)
	}
	if got := metricValue(report.Metrics, "coverage.branch.percent"); got != 50 {
		t.Fatalf("coverage.branch.percent = %v, want 50", got)
	}
}

func TestParseCodecovJSONReportsCoveragePercent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codecov.json")
	if err := os.WriteFile(path, []byte(`{"totals":{"coverage":87.5}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := parseCoverageJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := metricValue(report.Metrics, "coverage.line.percent"); got != 87.5 {
		t.Fatalf("coverage.line.percent = %v, want 87.5", got)
	}
}

func TestParseGoCycloReportsComplexityMetrics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gocyclo.txt")
	if err := os.WriteFile(
		path,
		[]byte("12 main hard internal/build/foo.go:10:1\n4 main easy internal/build/bar.go:20:1\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	report, err := parseGoCyclo(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := metricValue(report.Metrics, "complexity.max"); got != 12 {
		t.Fatalf("complexity.max = %v, want 12", got)
	}
	if got := metricValue(report.Metrics, "complexity.avg"); got != 8 {
		t.Fatalf("complexity.avg = %v, want 8", got)
	}
	if len(report.Findings) != 2 || report.Findings[0].File != "internal/build/foo.go" ||
		report.Findings[0].Line != 10 {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

func TestEvaluateQualityGatesFailsThresholds(t *testing.T) {
	min := 80.0
	max := 0.0
	gates := EvaluateGates(
		"run",
		"shell/test",
		[]model.QualityGateSpec{
			{Metric: "coverage.line.percent", Min: &min},
			{Metric: "tests.failed", Max: &max},
		},
		map[string]float64{"coverage.line.percent": 75, "tests.failed": 1},
	)
	if len(gates) != 2 || gates[0].Status != "failed" || gates[1].Status != "failed" {
		t.Fatalf("gates = %#v", gates)
	}
}

func metricValue(metrics []state.QualityMetric, name string) float64 {
	for _, metric := range metrics {
		if metric.Name == name {
			return metric.Value
		}
	}
	return 0
}
