package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runReport(args []string, stdin *strings.Reader, stdout *bytes.Buffer) error {
	cmd := newReportCommand(stdin, stdout)
	cmd.SetOut(stdout)
	cmd.SetErr(stdout)
	cmd.SetIn(stdin)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunReportFindingWritesArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	var stdout bytes.Buffer
	err := runReport([]string{
		"finding",
		"--path", path,
		"--allow-external-path",
		"--role", "docs-sweeper",
		"--kind", "docs",
		"--severity", "error",
		"--rule", "stale-reference",
		"--message", "docs are stale",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("runReport() error = %v", err)
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("stdout = %q, want report path", stdout.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var report struct {
		Schema   string                  `json:"schema"`
		Agent    struct{ Role string }   `json:"agent"`
		Status   string                  `json:"status"`
		Findings []struct{ Rule string } `json:"findings"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if report.Schema != "bach.agent_report.v1" ||
		report.Agent.Role != "docs-sweeper" ||
		report.Status != "success" {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Findings) != 1 || report.Findings[0].Rule != "stale-reference" {
		t.Fatalf("findings = %#v", report.Findings)
	}
}

func TestRunReportFindingStdinRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	var stdout bytes.Buffer
	err := runReport(
		[]string{"finding", "--path", path, "--allow-external-path", "--stdin"},
		strings.NewReader(`{"kind":"docs","extra":true}`),
		&stdout,
	)
	if err == nil {
		t.Fatalf("runReport() error = nil, want strict JSON error")
	}
}

func TestRunReportHelpPrintsUsage(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	if err := runReport([]string{"--help"}, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("runReport() error = %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"init",
		"finding",
		"bach.agent_report.v1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q:\n%s", want, got)
		}
	}
}

func TestRunReportSubcommandHelpPrintsFlags(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	if err := runReport([]string{"metric", "--help"}, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("runReport() error = %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"-name", "-value", "-path"} {
		if !strings.Contains(got, want) {
			t.Fatalf("metric help missing %q:\n%s", want, got)
		}
	}
}

func TestRunReportMetricUsesNameForMetricName(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	var stdout bytes.Buffer
	err := runReport([]string{
		"metric",
		"--path", path,
		"--allow-external-path",
		"--role", "demo",
		"--name", "demo.items.count",
		"--value", "3",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("runReport() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"name": "demo.items.count"`) {
		t.Fatalf("report missing metric name:\n%s", data)
	}
}

func TestRunReportStatusAcceptsStatusBeforeFlags(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agent-report-v1.json")
	var stdout bytes.Buffer
	err := runReport([]string{
		"status",
		"success",
		"--path", path,
		"--allow-external-path",
		"--role", "demo",
		"--summary", "done",
	}, strings.NewReader(""), &stdout)
	if err != nil {
		t.Fatalf("runReport() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"summary": "done"`) {
		t.Fatalf("report missing status summary:\n%s", data)
	}
}
