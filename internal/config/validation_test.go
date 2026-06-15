package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWithOptionsSkipsPluginExecution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "test" {
  command = ["true"]
}

plugin "bad" {
  type    = "graph"
  command = ["sh", "-c", "exit 7"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	report := ValidateWithOptions(path, LoadOptions{})
	if !report.Valid {
		t.Fatalf("report = %#v, want valid because validation skips plugin execution", report)
	}
	if report.Summary.Targets != 1 {
		t.Fatalf("targets = %d, want 1", report.Summary.Targets)
	}
}

func TestValidateWithOptionsReportsParseDiagnosticRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	if err := os.WriteFile(path, []byte("project \"example\" {\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := ValidateWithOptions(path, LoadOptions{})
	if report.Valid || len(report.Diagnostics) == 0 {
		t.Fatalf("report = %#v, want parse diagnostic", report)
	}
	diag := report.Diagnostics[0]
	if diag.File != path || diag.Range.Start.Line == 0 || diag.Code != "hcl-parse-error" {
		t.Fatalf("diagnostic = %#v", diag)
	}
}
