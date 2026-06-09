package quality

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/state"
)

func TestIngestReportsPersistsAndFailsGate(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	if err := os.WriteFile(
		filepath.Join(dir, "coverage.out"),
		[]byte("mode: set\nexample.go:1.1,2.1 2 1\nexample.go:3.1,4.1 2 0\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	min := 80.0
	err := IngestReports(context.Background(), IngestRequest{
		StatePath:  statePath,
		RunID:      "run-1",
		TargetName: "shell/test",
		Workdir:    dir,
		Env:        map[string]string{},
		Reports: []model.QualityReportDeclaration{
			{Kind: "coverage", Format: "go-cover", Path: "coverage.out"},
		},
		Gates: []model.QualityGateSpec{{Metric: "coverage.line.percent", Min: &min}},
		Log:   &strings.Builder{},
	})
	if err == nil || !strings.Contains(err.Error(), "quality gates failed") {
		t.Fatalf("err = %v, want quality gate failure", err)
	}
	if !IsGateError(err) {
		t.Fatalf("IsGateError(%T) = false", err)
	}
	reports, err := state.NewStore(statePath).ListQualityReports(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].Status != "success" {
		t.Fatalf("reports = %#v", reports)
	}
	gates, err := state.NewStore(statePath).ListQualityGateResults(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 1 || gates[0].Status != "failed" {
		t.Fatalf("gates = %#v", gates)
	}
}

func TestIngestReportsPersistsFailedReport(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	var log strings.Builder
	err := IngestReports(context.Background(), IngestRequest{
		StatePath:  statePath,
		RunID:      "run-1",
		TargetName: "shell/test",
		Workdir:    dir,
		Reports: []model.QualityReportDeclaration{
			{Kind: "coverage", Format: "go-cover", Path: "missing.out"},
		},
		Log: &log,
	})
	if err != nil {
		t.Fatal(err)
	}
	reports, err := state.NewStore(statePath).ListQualityReports(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].Status != "failed" || reports[0].Message == "" {
		t.Fatalf("reports = %#v", reports)
	}
	if !strings.Contains(log.String(), "quality report missing.out failed") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestIngestReportsNoopWithoutReportsOrGates(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	if err := IngestReports(context.Background(), IngestRequest{StatePath: statePath}); err != nil {
		t.Fatal(err)
	}
	reports, err := state.NewStore(statePath).ListQualityReports(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("reports = %#v", reports)
	}
}
