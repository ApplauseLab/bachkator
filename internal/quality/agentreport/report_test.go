package agentreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAppendFindingAutoCreatesReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	written, err := AppendFinding(Defaults{Path: path, AllowExternalPath: true}, Finding{
		Kind:     "docs",
		Severity: "error",
		Rule:     "stale-reference",
		Message:  "docs are stale",
	})
	if err != nil {
		t.Fatalf("AppendFinding() error = %v", err)
	}
	if written != path {
		t.Fatalf("written path = %q, want %q", written, path)
	}
	report := readReport(t, path)
	if report.Schema != Schema || report.Agent.Role != "reporter" || report.Status != "success" {
		t.Fatalf("report defaults = %#v", report)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != "stale-reference" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

func TestAppendMetricPreservesExistingReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	if _, err := WriteInit(Defaults{
		Path:              path,
		Role:              "docs-sweeper",
		Summary:           "started",
		AllowExternalPath: true,
	}); err != nil {
		t.Fatalf("WriteInit() error = %v", err)
	}
	if _, err := AppendMetric(Defaults{Path: path, AllowExternalPath: true}, Metric{
		Name:  "review.docs.checked_files.count",
		Value: 12,
		Unit:  "count",
	}); err != nil {
		t.Fatalf("AppendMetric() error = %v", err)
	}
	report := readReport(t, path)
	if report.Agent.Role != "docs-sweeper" || report.Summary != "started" {
		t.Fatalf("report identity = %#v", report)
	}
	if len(report.Metrics) != 1 || report.Metrics[0].Value != 12 {
		t.Fatalf("metrics = %#v", report.Metrics)
	}
}

func TestStatusUpdatesReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	if _, err := UpdateStatus(Defaults{
		Path:              path,
		Summary:           "failed review",
		AllowExternalPath: true,
	}, "failed"); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	report := readReport(t, path)
	if report.Status != "failed" || report.Summary != "failed review" {
		t.Fatalf("status report = %#v", report)
	}
}

func TestDecodeFindingStrictRejectsUnknownFields(t *testing.T) {
	if _, err := DecodeFindingStrict([]byte(`{"kind":"docs","extra":true}`)); err == nil {
		t.Fatalf("DecodeFindingStrict() error = nil, want unknown field error")
	}
}

func TestDecodeFindingStrictRejectsTrailingJSON(t *testing.T) {
	if _, err := DecodeFindingStrict([]byte(`{"kind":"docs"} {"kind":"ignored"}`)); err == nil {
		t.Fatalf("DecodeFindingStrict() error = nil, want trailing JSON error")
	}
}

func TestResolvePathOrder(t *testing.T) {
	env := map[string]string{
		"BACH_AGENT_QUALITY_REPORT_PATH": "env-report.json",
		"BACH_RUN_DIRECTORY":             "run-dir",
	}
	if path, err := ResolvePath("explicit.json", env); err != nil || path != "explicit.json" {
		t.Fatalf("explicit ResolvePath() = %q, %v", path, err)
	}
	if path, err := ResolvePath("", env); err != nil || path != "env-report.json" {
		t.Fatalf("env ResolvePath() = %q, %v", path, err)
	}
	delete(env, "BACH_AGENT_QUALITY_REPORT_PATH")
	want := filepath.Join("run-dir", "agent-report-v1.json")
	if path, err := ResolvePath("", env); err != nil || path != want {
		t.Fatalf("run-dir ResolvePath() = %q, %v", path, err)
	}
}

func TestConcurrentAppendKeepsValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := AppendFinding(
				Defaults{Path: path, AllowExternalPath: true},
				Finding{Kind: "docs"},
			)
			if err != nil {
				t.Errorf("AppendFinding() error = %v", err)
			}
		}()
	}
	wg.Wait()
	report := readReport(t, path)
	if len(report.Findings) != 10 {
		t.Fatalf("findings = %d, want 10", len(report.Findings))
	}
}

func TestAppendFindingRejectsSymlinkEscape(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(workspace, "reports")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Chdir(workspace)
	_, err := AppendFinding(
		Defaults{Path: filepath.Join("reports", "agent-report-v1.json")},
		Finding{Kind: "docs"},
	)
	if err == nil {
		t.Fatalf("AppendFinding() error = nil, want symlink escape rejection")
	}
}

func readReport(t *testing.T, path string) Report {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return report
}
