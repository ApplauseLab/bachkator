package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDecodesPipelineTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile.hcl")
	contents := `project "example" {
  default = "pipeline.deploy-staging"
}

shell "render-staging" {
  command = ["true"]
}

shell "apply-staging" {
  command = ["true"]
}

pipeline "deploy-staging" {
  steps = [
    shell.render-staging,
    "shell.apply-staging",
  ]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	target := project.Targets["pipeline/deploy-staging"]
	if target == nil {
		t.Fatal("missing pipeline target")
	}
	if got := target.Steps; len(got) != 2 || got[0] != "shell/render-staging" ||
		got[1] != "shell/apply-staging" {
		t.Fatalf("pipeline steps = %v, want render/apply", got)
	}
}
func TestLoadDecodesGroupTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile.hcl")
	contents := `project "example" {
  default = "group.ci"
}

shell "lint" {
  command = ["true"]
}

shell "test" {
  command = ["true"]
}

group "ci" {
  targets = [shell.lint, "shell.test"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	target := project.Targets["group/ci"]
	if target == nil {
		t.Fatal("missing group target")
	}
	if got := target.Targets; len(got) != 2 || got[0] != "shell/lint" || got[1] != "shell/test" {
		t.Fatalf("group targets = %v, want lint/test", got)
	}
	if project.DefaultTarget != "group/ci" {
		t.Fatalf("default target = %q, want group/ci", project.DefaultTarget)
	}
}
func TestLoadRejectsPipelineMissingStep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile.hcl")
	contents := `project "example" {}

pipeline "deploy-staging" {
  steps = ["shell.missing"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected missing pipeline step error")
	}
}
func TestLoadAllowsNestedPipelineStep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile.hcl")
	contents := `project "example" {}

shell "render" {
  command = ["true"]
}

pipeline "inner" {
  steps = ["shell.render"]
}

pipeline "outer" {
  steps = [pipeline.inner]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := project.Targets["pipeline/outer"].Steps; len(got) != 1 || got[0] != "pipeline/inner" {
		t.Fatalf("outer pipeline steps = %v, want pipeline/inner", got)
	}
}
func TestLoadRejectsPipelineStepCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile.hcl")
	contents := `project "example" {}

pipeline "inner" {
  steps = ["pipeline.outer"]
}

pipeline "outer" {
  steps = ["pipeline.inner"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected pipeline cycle error")
	} else if !strings.Contains(err.Error(), `composite target cycle includes`) {
		t.Fatalf("error = %q, want composite target cycle", err.Error())
	}
}
