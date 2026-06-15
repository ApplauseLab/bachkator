package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCommandReportsValidProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

input "file" "source" {
  src = "source.txt"
}

alias "default" {
  target = "shell.test"
}

profile "ci" {
  env {
    MODE = "ci"
  }
}

shell "test" {
  command = ["true"]
}
`)

	var stdout bytes.Buffer
	err := Execute(
		context.Background(),
		[]string{"-f", filepath.Join(dir, "Bachfile"), "validate"},
		&stdout,
		&bytes.Buffer{},
		"test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); !strings.Contains(
		got,
		"Bachfile valid: 1 targets, 1 aliases, 1 inputs, 1 profiles",
	) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestValidateCommandJSONReportsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {
  default = "shell/test"
}

shell "test" {
  command = ["true"]
}
`)

	var stdout bytes.Buffer
	err := Execute(
		context.Background(),
		[]string{"-f", filepath.Join(dir, "Bachfile"), "--json", "validate"},
		&stdout,
		&bytes.Buffer{},
		"test",
	)
	if err == nil {
		t.Fatal("validate succeeded, want validation failure")
	}
	var report ValidationReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if report.Valid || len(report.Diagnostics) != 1 {
		t.Fatalf("report = %#v, want one invalid diagnostic", report)
	}
	if got := report.Diagnostics[0].Code; got != "obsolete-target-reference" {
		t.Fatalf("code = %q", got)
	}
}
