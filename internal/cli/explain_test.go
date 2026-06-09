package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainPrintsTargetGuidance(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "deploy-staging" {
  description = "Deploy staging API"
  when        = "after image publish"
  cost        = "high"
  remote      = true
  inputs      = ["dist/image.txt"]
  outputs     = ["release/staging.txt"]
  command     = ["true"]
}
`)

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "explain", "shell/deploy-staging"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{"Target: shell/deploy-staging", "When: after image publish", "Cost: high", "Risks: remote", "  - dist/image.txt", "  - release/staging.txt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestListVerbosePrintsCostAndRiskColumns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "deploy-staging" {
  description = "Deploy staging API"
  cost        = "high"
  remote      = true
  destructive = true
  command     = ["true"]
}
`)

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "list", "-verbose"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{"TARGET", "COST", "RISKS", "shell/deploy-staging", "high", "remote,destructive", "Deploy staging API"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestListVerbosePrintsInheritedRisks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "apply-staging" {
  remote                = true
  requires_confirmation = true
  command               = ["true"]
}

pipeline "deploy-staging" {
  steps = [shell.apply-staging]
}
`)

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "list", "-verbose"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	if !strings.Contains(got, "pipeline/deploy-staging") ||
		!strings.Contains(got, "remote,requires_confirmation") {
		t.Fatalf("stdout missing inherited risks:\n%s", got)
	}
}

func TestYesFlagConfirmsRiskyTargetExecution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "deploy-staging" {
  requires_confirmation = true
  shell                 = "printf deploy > deployed.txt"
}
`)

	args := []string{"-f", filepath.Join(dir, "Bachfile"), "-yes", "run", "shell/deploy-staging"}
	if err := Execute(
		context.Background(),
		args,
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "deployed.txt")); err != nil {
		t.Fatalf("expected confirmed target output: %v", err)
	}
}

func TestListAliasesPrintsAliasMappings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

alias "old-deploy" {
  target      = "shell.deploy-staging"
  deprecated = "Use shell.deploy-staging."
}

shell "deploy-staging" {
  command = ["true"]
}
`)

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "list", "-aliases"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{"shell/deploy-staging", "old-deploy -> shell/deploy-staging", "Use shell.deploy-staging."} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestExplainAliasPrintsCanonicalTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

alias "old-test" {
  target      = "shell.test"
  deprecated = "Use shell.test."
}

shell "test" {
  command = ["true"]
}
`)

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "explain", "old-test"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{"Alias: old-test", "Canonical target: shell/test", "Deprecated: Use shell.test.", "Target: shell/test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestDryRunAliasUsesCanonicalTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

alias "old-test" {
  target      = "shell.test"
  deprecated = "Use shell.test."
}

shell "test" {
  command = ["printf", "ok"]
}
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "-dry-run", "run", "old-test"}
	if err := Execute(context.Background(), args, &stdout, &stderr, "test"); err != nil {
		t.Fatal(err)
	}

	if got := stdout.String(); !strings.Contains(got, "[shell/test]") ||
		strings.Contains(got, "old-test") {
		t.Fatalf("stdout = %q, want canonical target only", got)
	}
	for _, want := range []string{"alias \"old-test\" resolves to \"shell/test\"", "Use shell.test."} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}
