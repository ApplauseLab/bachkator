package quality

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/state"
)

type junitTestCase struct {
	Classname string  `xml:"classname,attr"`
	Name      string  `xml:"name,attr"`
	Time      float64 `xml:"time,attr"`
	Failure   []struct {
		Message string `xml:"message,attr"`
	} `xml:"failure"`
	Error []struct {
		Message string `xml:"message,attr"`
	} `xml:"error"`
}

type junitTestSuite struct {
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Errors   int              `xml:"errors,attr"`
	Skipped  int              `xml:"skipped,attr"`
	Time     float64          `xml:"time,attr"`
	Cases    []junitTestCase  `xml:"testcase"`
	Suites   []junitTestSuite `xml:"testsuite"`
}

type junitTestSuites struct {
	Suites []junitTestSuite `xml:"testsuite"`
}

func EvaluateGates(
	runID string,
	targetName string,
	gates []model.QualityGateSpec,
	metrics map[string]float64,
) []state.QualityGateResult {
	evaluator, err := BuiltinGateRegistry().Evaluator("threshold")
	if err != nil {
		return []state.QualityGateResult{
			{
				RunID:     runID,
				Target:    targetName,
				Status:    "failed",
				Message:   err.Error(),
				CreatedAt: time.Now().UTC(),
			},
		}
	}
	return evaluator(runID, targetName, gates, metrics)
}

func evaluateThresholdGates(
	runID string,
	targetName string,
	gates []model.QualityGateSpec,
	metrics map[string]float64,
) []state.QualityGateResult {
	results := make([]state.QualityGateResult, 0, len(gates)*2)
	for _, gate := range gates {
		actual, ok := metrics[gate.Metric]
		if !ok {
			results = append(
				results,
				state.QualityGateResult{
					RunID:     runID,
					Target:    targetName,
					Metric:    gate.Metric,
					Op:        "exists",
					Status:    "failed",
					Message:   fmt.Sprintf("metric %q was not reported", gate.Metric),
					CreatedAt: time.Now().UTC(),
				},
			)
			continue
		}
		if gate.Min != nil {
			status := "success"
			if actual < *gate.Min {
				status = "failed"
			}
			results = append(
				results,
				state.QualityGateResult{
					RunID:     runID,
					Target:    targetName,
					Metric:    gate.Metric,
					Op:        ">=",
					Threshold: *gate.Min,
					Actual:    actual,
					Status:    status,
					Message: fmt.Sprintf(
						"%s actual %.3f must be >= %.3f",
						gate.Metric,
						actual,
						*gate.Min,
					),
					CreatedAt: time.Now().UTC(),
				},
			)
		}
		if gate.Max != nil {
			status := "success"
			if actual > *gate.Max {
				status = "failed"
			}
			results = append(
				results,
				state.QualityGateResult{
					RunID:     runID,
					Target:    targetName,
					Metric:    gate.Metric,
					Op:        "<=",
					Threshold: *gate.Max,
					Actual:    actual,
					Status:    status,
					Message: fmt.Sprintf(
						"%s actual %.3f must be <= %.3f",
						gate.Metric,
						actual,
						*gate.Max,
					),
					CreatedAt: time.Now().UTC(),
				},
			)
		}
	}
	return results
}

func ParseReport(
	path string,
	declaration model.QualityReportDeclaration,
) (state.QualityReport, error) {
	parser, err := BuiltinReportParserRegistry().Parser(declaration.Format)
	if err != nil {
		return state.QualityReport{}, err
	}
	return parser.Parse(path)
}

func parseJUnitXML(path string) (state.QualityReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	var rootSuites junitTestSuites
	if err := xml.Unmarshal(data, &rootSuites); err == nil && len(rootSuites.Suites) > 0 {
		return junitFromSuites(rootSuites.Suites), nil
	}
	var suite junitTestSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return state.QualityReport{}, err
	}
	return junitFromSuites([]junitTestSuite{suite}), nil
}

func junitFromSuites(suites []junitTestSuite) state.QualityReport {
	var tests, failures, errors, skipped int
	var duration float64
	findings := []state.QualityFinding{}
	var walk func(junitTestSuite)
	walk = func(suite junitTestSuite) {
		tests += suite.Tests
		failures += suite.Failures
		errors += suite.Errors
		skipped += suite.Skipped
		duration += suite.Time
		for _, test := range suite.Cases {
			name := test.Name
			if test.Classname != "" {
				name = test.Classname + "." + test.Name
			}
			findings = append(
				findings,
				state.QualityFinding{Kind: "test", Rule: name, DurationMS: test.Time * 1000},
			)
			for _, failure := range test.Failure {
				findings = append(
					findings,
					state.QualityFinding{
						Kind:       "test-failure",
						Severity:   "failure",
						Rule:       name,
						Message:    failure.Message,
						DurationMS: test.Time * 1000,
					},
				)
			}
			for _, testError := range test.Error {
				findings = append(
					findings,
					state.QualityFinding{
						Kind:       "test-failure",
						Severity:   "error",
						Rule:       name,
						Message:    testError.Message,
						DurationMS: test.Time * 1000,
					},
				)
			}
		}
		for _, child := range suite.Suites {
			walk(child)
		}
	}
	for _, suite := range suites {
		walk(suite)
	}
	return state.QualityReport{Metrics: []state.QualityMetric{
		{Name: "tests.total", Value: float64(tests), Unit: "count"},
		{Name: "tests.failed", Value: float64(failures + errors), Unit: "count"},
		{Name: "tests.skipped", Value: float64(skipped), Unit: "count"},
		{Name: "tests.duration.ms", Value: duration * 1000, Unit: "ms"},
	}, Findings: findings}
}

func parseCheckstyleXML(path string) (state.QualityReport, error) {
	type checkError struct {
		Line     int    `xml:"line,attr"`
		Severity string `xml:"severity,attr"`
		Message  string `xml:"message,attr"`
		Source   string `xml:"source,attr"`
	}
	type checkFile struct {
		Name   string       `xml:"name,attr"`
		Errors []checkError `xml:"error"`
	}
	type checkstyle struct {
		Files []checkFile `xml:"file"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	var root checkstyle
	if err := xml.Unmarshal(data, &root); err != nil {
		return state.QualityReport{}, err
	}
	var findings []state.QualityFinding
	severityCounts := map[string]float64{}
	for _, file := range root.Files {
		for _, issue := range file.Errors {
			severity := issue.Severity
			if severity == "" {
				severity = "warning"
			}
			severityCounts[severity]++
			findings = append(
				findings,
				state.QualityFinding{
					Kind:     "issue",
					File:     file.Name,
					Line:     issue.Line,
					Severity: severity,
					Rule:     issue.Source,
					Message:  issue.Message,
				},
			)
		}
	}
	metrics := []state.QualityMetric{
		{Name: "issues.total.count", Value: float64(len(findings)), Unit: "count"},
	}
	for severity, count := range severityCounts {
		metrics = append(
			metrics,
			state.QualityMetric{Name: "issues." + severity + ".count", Value: count, Unit: "count"},
		)
	}
	return state.QualityReport{Metrics: metrics, Findings: findings}, nil
}

func parseLCOV(path string) (state.QualityReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	defer func() { _ = file.Close() }()
	var found, hit float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "LF:") {
			found += parseFloat(strings.TrimPrefix(line, "LF:"))
		}
		if strings.HasPrefix(line, "LH:") {
			hit += parseFloat(strings.TrimPrefix(line, "LH:"))
		}
	}
	if err := scanner.Err(); err != nil {
		return state.QualityReport{}, err
	}
	percent := 0.0
	if found > 0 {
		percent = hit / found * 100
	}
	return state.QualityReport{
		Metrics: []state.QualityMetric{
			{Name: "coverage.line.percent", Value: percent, Unit: "percent"},
			{Name: "coverage.lines.covered", Value: hit, Unit: "count"},
			{Name: "coverage.lines.total", Value: found, Unit: "count"},
		},
	}, nil
}

func parseCoberturaXML(path string) (state.QualityReport, error) {
	type coverage struct {
		LineRate     string `xml:"line-rate,attr"`
		BranchRate   string `xml:"branch-rate,attr"`
		LinesValid   string `xml:"lines-valid,attr"`
		LinesCovered string `xml:"lines-covered,attr"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	var root coverage
	if err := xml.Unmarshal(data, &root); err != nil {
		return state.QualityReport{}, err
	}
	linePercent := parseFloat(root.LineRate) * 100
	branchPercent := parseFloat(root.BranchRate) * 100
	return state.QualityReport{Metrics: []state.QualityMetric{
		{Name: "coverage.line.percent", Value: linePercent, Unit: "percent"},
		{Name: "coverage.branch.percent", Value: branchPercent, Unit: "percent"},
		{Name: "coverage.lines.covered", Value: parseFloat(root.LinesCovered), Unit: "count"},
		{Name: "coverage.lines.total", Value: parseFloat(root.LinesValid), Unit: "count"},
	}}, nil
}

func parseJaCoCoXML(path string) (state.QualityReport, error) {
	type counter struct {
		Type    string `xml:"type,attr"`
		Missed  string `xml:"missed,attr"`
		Covered string `xml:"covered,attr"`
	}
	type report struct {
		Counters []counter `xml:"counter"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	var root report
	if err := xml.Unmarshal(data, &root); err != nil {
		return state.QualityReport{}, err
	}
	metrics := []state.QualityMetric{}
	for _, item := range root.Counters {
		covered := parseFloat(item.Covered)
		missed := parseFloat(item.Missed)
		total := covered + missed
		percent := 0.0
		if total > 0 {
			percent = covered / total * 100
		}
		switch item.Type {
		case "LINE":
			metrics = append(
				metrics,
				state.QualityMetric{Name: "coverage.line.percent", Value: percent, Unit: "percent"},
				state.QualityMetric{Name: "coverage.lines.covered", Value: covered, Unit: "count"},
				state.QualityMetric{Name: "coverage.lines.total", Value: total, Unit: "count"},
			)
		case "BRANCH":
			metrics = append(
				metrics,
				state.QualityMetric{
					Name:  "coverage.branch.percent",
					Value: percent,
					Unit:  "percent",
				},
			)
		case "COMPLEXITY":
			metrics = append(
				metrics,
				state.QualityMetric{Name: "complexity.total", Value: total, Unit: "count"},
			)
		}
	}
	if len(metrics) == 0 {
		return state.QualityReport{}, fmt.Errorf("JaCoCo XML did not contain supported counters")
	}
	return state.QualityReport{Metrics: metrics}, nil
}

func parseGoCover(path string) (state.QualityReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	defer func() { _ = file.Close() }()
	var statements, covered float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		count := parseFloat(parts[1])
		statements += count
		if parseFloat(parts[2]) > 0 {
			covered += count
		}
	}
	if err := scanner.Err(); err != nil {
		return state.QualityReport{}, err
	}
	percent := 0.0
	if statements > 0 {
		percent = covered / statements * 100
	}
	return state.QualityReport{
		Metrics: []state.QualityMetric{
			{Name: "coverage.line.percent", Value: percent, Unit: "percent"},
			{Name: "coverage.lines.covered", Value: covered, Unit: "count"},
			{Name: "coverage.lines.total", Value: statements, Unit: "count"},
		},
	}, nil
}

func parseCoverageJSON(path string) (state.QualityReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return state.QualityReport{}, err
	}
	metrics := []state.QualityMetric{}
	for _, key := range []string{"coverage", "line_coverage", "line_percent"} {
		if value, ok := numberFromJSON(values[key]); ok {
			metrics = append(
				metrics,
				state.QualityMetric{Name: "coverage.line.percent", Value: value, Unit: "percent"},
			)
			break
		}
	}
	if totals, ok := values["totals"].(map[string]any); ok {
		for _, key := range []string{"coverage", "line_coverage", "line_percent"} {
			if value, ok := numberFromJSON(totals[key]); ok {
				metrics = append(
					metrics,
					state.QualityMetric{
						Name:  "coverage.line.percent",
						Value: value,
						Unit:  "percent",
					},
				)
				break
			}
		}
	}
	if len(metrics) == 0 {
		return state.QualityReport{}, fmt.Errorf(
			"coverage JSON did not contain coverage, line_coverage, line_percent, or totals coverage",
		)
	}
	return state.QualityReport{Metrics: metrics}, nil
}

func parseGoCyclo(path string) (state.QualityReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return state.QualityReport{}, err
	}
	defer func() { _ = file.Close() }()
	var count, total, max float64
	findings := []state.QualityFinding{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		complexity := parseFloat(parts[0])
		count++
		total += complexity
		if complexity > max {
			max = complexity
		}
		file, lineNumber := splitLocation(parts[len(parts)-1])
		findings = append(
			findings,
			state.QualityFinding{
				Kind:       "complexity",
				File:       file,
				Line:       lineNumber,
				Severity:   "info",
				Rule:       parts[2],
				Message:    fmt.Sprintf("cyclomatic complexity %.0f", complexity),
				DurationMS: complexity,
			},
		)
	}
	if err := scanner.Err(); err != nil {
		return state.QualityReport{}, err
	}
	avg := 0.0
	if count > 0 {
		avg = total / count
	}
	return state.QualityReport{
		Metrics: []state.QualityMetric{
			{Name: "complexity.max", Value: max, Unit: "count"},
			{Name: "complexity.avg", Value: avg, Unit: "count"},
			{Name: "complexity.functions.count", Value: count, Unit: "count"},
		},
		Findings: findings,
	}, nil
}

func splitLocation(location string) (string, int) {
	parts := strings.Split(location, ":")
	if len(parts) < 2 {
		return location, 0
	}
	line, _ := strconv.Atoi(parts[len(parts)-2])
	return strings.Join(parts[:len(parts)-2], ":"), line
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func numberFromJSON(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
