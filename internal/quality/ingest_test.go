package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality/agentreport"
)

type savedQuality struct {
	reports []Report
	gates   []GateResult
}

func (s *savedQuality) save(_ context.Context, reports []Report, gates []GateResult) error {
	s.reports = append([]Report(nil), reports...)
	s.gates = append([]GateResult(nil), gates...)
	return nil
}

type fakeParsers struct{}

func (fakeParsers) Parser(format string) (Parser, error) {
	if format != "fake-format" {
		return nil, fmt.Errorf("unexpected parser format %q", format)
	}
	return parserFunc(func(string) (Report, error) {
		return Report{Metrics: []Metric{{Name: "fake.metric", Value: 7}}}, nil
	}), nil
}

type fakeGateEvaluators struct{}

func (fakeGateEvaluators) Evaluator(name string) (GateEvaluator, error) {
	if name != "threshold" {
		return nil, fmt.Errorf("unexpected evaluator %q", name)
	}
	return func(
		runID string,
		targetName string,
		_ []model.QualityGateSpec,
		metrics map[string]float64,
	) []GateResult {
		return []GateResult{{
			RunID:  runID,
			Target: targetName,
			Metric: "fake.metric",
			Actual: metrics["fake.metric"],
			Status: "success",
		}}
	}, nil
}

func TestIngestReportsUsesInjectedQualityRegistries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var saved savedQuality
	now := time.Date(2026, 6, 14, 9, 30, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(dir, "fake.out"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := IngestReports(context.Background(), IngestRequest{
		RunID:      "run-1",
		TargetName: "shell/test",
		Workdir:    dir,
		Env:        map[string]string{},
		Reports: []model.QualityReportDeclaration{{
			Kind:   "fake",
			Format: "fake-format",
			Path:   "fake.out",
		}},
		Parsers:        fakeParsers{},
		Gates:          []model.QualityGateSpec{{Metric: "fake.metric"}},
		GateEvaluators: fakeGateEvaluators{},
		SaveReports:    saved.save,
		Log:            &strings.Builder{},
		Now:            func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	gates := saved.gates
	if len(gates) != 1 || gates[0].Actual != 7 || gates[0].Status != "success" {
		t.Fatalf("gates = %#v, want injected evaluator result", gates)
	}
	if len(saved.reports) != 1 || !saved.reports[0].CreatedAt.Equal(now) ||
		!gates[0].CreatedAt.Equal(now) {
		t.Fatalf("timestamps reports=%#v gates=%#v, want %s", saved.reports, gates, now)
	}
}

func TestIngestReportsPersistsAndFailsGate(t *testing.T) {
	dir := t.TempDir()
	var saved savedQuality
	if err := os.WriteFile(
		filepath.Join(dir, "coverage.out"),
		[]byte("mode: set\nexample.go:1.1,2.1 2 1\nexample.go:3.1,4.1 2 0\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	min := 80.0
	err := IngestReports(context.Background(), IngestRequest{
		RunID:      "run-1",
		TargetName: "shell/test",
		Workdir:    dir,
		Env:        map[string]string{},
		Reports: []model.QualityReportDeclaration{
			{Kind: "coverage", Format: "go-cover", Path: "coverage.out"},
		},
		Gates:       []model.QualityGateSpec{{Metric: "coverage.line.percent", Min: &min}},
		SaveReports: saved.save,
		Log:         &strings.Builder{},
	})
	if err == nil || !strings.Contains(err.Error(), "quality gates failed") {
		t.Fatalf("err = %v, want quality gate failure", err)
	}
	if !IsGateError(err) {
		t.Fatalf("IsGateError(%T) = false", err)
	}
	reports := saved.reports
	if len(reports) != 1 || reports[0].Status != "success" {
		t.Fatalf("reports = %#v", reports)
	}
	gates := saved.gates
	if len(gates) != 1 || gates[0].Status != "failed" {
		t.Fatalf("gates = %#v", gates)
	}
}

func TestIngestReportsPersistsFailedReport(t *testing.T) {
	dir := t.TempDir()
	var saved savedQuality
	var log strings.Builder
	err := IngestReports(context.Background(), IngestRequest{
		RunID:      "run-1",
		TargetName: "shell/test",
		Workdir:    dir,
		Reports: []model.QualityReportDeclaration{
			{Kind: "coverage", Format: "go-cover", Path: "missing.out"},
		},
		SaveReports: saved.save,
		Log:         &log,
	})
	if !IsParseError(err) {
		t.Fatalf("err = %v, want parse error", err)
	}
	reports := saved.reports
	if len(reports) != 1 || reports[0].Status != "failed" || reports[0].Message == "" {
		t.Fatalf("reports = %#v", reports)
	}
	if !strings.Contains(log.String(), "quality report missing.out failed") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestIngestReportsNoopWithoutReportsOrGates(t *testing.T) {
	var saved savedQuality
	if err := IngestReports(context.Background(), IngestRequest{
		SaveReports: saved.save,
	}); err != nil {
		t.Fatal(err)
	}
	reports := saved.reports
	if len(reports) != 0 {
		t.Fatalf("reports = %#v", reports)
	}
}

func TestIngestReportsEvaluatesDenyingRegoPolicy(t *testing.T) {
	dir := t.TempDir()
	var saved savedQuality
	writeAgentReport(t, filepath.Join(dir, "security.json"), []agentreport.Finding{{
		Kind:     "security",
		Severity: "critical",
		Rule:     "CVE-2026-1234",
		Message:  "openssl has critical vulnerability",
		File:     "go.sum",
		Line:     42,
	}})
	policyPath := filepath.Join(dir, "no-critical.rego")
	if err := os.WriteFile(policyPath, []byte(`package bach.policy

default allow := true

allow := false if {
  some i
  f := input.findings[i]
  f.kind == "security"
  f.severity == "critical"
}

findings := [finding |
  some i
  f := input.findings[i]
  f.kind == "security"
  f.severity == "critical"
  finding := {
    "kind": "security-policy",
    "severity": "error",
    "rule": "no-critical-security-findings",
    "message": sprintf("Critical security finding: %s", [f.message]),
    "file": f.file,
    "line": f.line,
  }
]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	err := IngestReports(context.Background(), IngestRequest{
		RunID:       "run-1",
		TargetName:  "shell/security-scan",
		ProjectRoot: dir,
		Workdir:     dir,
		Env:         map[string]string{},
		Reports: []model.QualityReportDeclaration{
			{Kind: "security", Format: "agent-report-v1", Path: "security.json"},
		},
		RegoPolicies: []model.RegoPolicySpec{{Path: "no-critical.rego", Package: "bach.policy"}},
		SaveReports:  saved.save,
		Log:          &strings.Builder{},
	})
	if err == nil || !IsGateError(err) {
		t.Fatalf("err = %v, want rego gate failure", err)
	}
	reports := saved.reports
	var policyReport *Report
	for i := range reports {
		if reports[i].Format == "rego-policy-v1" {
			policyReport = &reports[i]
		}
	}
	if policyReport == nil || policyReport.Status != "failed" {
		t.Fatalf("policy report = %#v", policyReport)
	}
	var findings []Finding
	for _, report := range reports {
		findings = append(findings, report.Findings...)
	}
	foundPolicyFinding := false
	for _, finding := range findings {
		if finding.Kind == "security-policy" && finding.File == "go.sum" {
			foundPolicyFinding = true
		}
	}
	if !foundPolicyFinding {
		t.Fatalf("policy findings = %#v", findings)
	}
}

func TestIngestReportsRejectsNonBooleanRegoAllow(t *testing.T) {
	dir := t.TempDir()
	var saved savedQuality
	if err := os.WriteFile(filepath.Join(dir, "bad.rego"), []byte(`package bach.policy

allow := "yes"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	err := IngestReports(context.Background(), IngestRequest{
		RunID:        "run-1",
		TargetName:   "shell/scan",
		ProjectRoot:  dir,
		Workdir:      dir,
		Env:          map[string]string{},
		RegoPolicies: []model.RegoPolicySpec{{Path: "bad.rego", Package: "bach.policy"}},
		SaveReports:  saved.save,
		Log:          &strings.Builder{},
	})
	if err == nil || !strings.Contains(err.Error(), "quality gates failed") {
		t.Fatalf("err = %v, want quality gate failure", err)
	}
	reports := saved.reports
	if len(reports) != 1 || reports[0].Status != "failed" ||
		!strings.Contains(reports[0].Message, "allow must evaluate to a boolean") {
		t.Fatalf("reports = %#v", reports)
	}
}

func writeAgentReport(t *testing.T, path string, findings []agentreport.Finding) {
	t.Helper()
	report := agentreport.Report{
		Schema:   agentreport.Schema,
		Agent:    agentreport.Actor{Role: "security"},
		Status:   "success",
		Summary:  "security scan",
		Metrics:  []agentreport.Metric{},
		Findings: findings,
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
