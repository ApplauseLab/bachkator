package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDecodesTargetMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "deploy-staging" {
  description           = "Deploy staging API"
  when                  = "after image publish"
  cost                  = "high"
  remote                = true
  destructive           = true
  requires_confirmation = true
  command               = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	target := project.Targets["shell/deploy-staging"]
	if target.Description != "Deploy staging API" || target.When != "after image publish" ||
		target.Cost != "high" {
		t.Fatalf("metadata = %#v", target)
	}
	if !target.Remote || !target.Destructive || !target.RequiresConfirmation {
		t.Fatalf(
			"risk metadata = remote:%v destructive:%v confirmation:%v",
			target.Remote,
			target.Destructive,
			target.RequiresConfirmation,
		)
	}
}
func TestLoadDecodesTargetTools(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "deploy-staging" {
  tools = [
    { name = "kubectl", command = ["kubectl", "version", "--client"], version = "client v1.30", fix = "Install kubectl." },
    { name = "aws" },
  ]
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	tools := project.Targets["shell/deploy-staging"].Spec().Runtime.Tools
	if len(tools) != 2 {
		t.Fatalf("tools = %#v, want 2 entries", tools)
	}
	if tools[0].Name != "kubectl" ||
		strings.Join(tools[0].Command, " ") != "kubectl version --client" ||
		tools[0].Version != "client v1.30" ||
		tools[0].Fix != "Install kubectl." {
		t.Fatalf("first tool = %#v", tools[0])
	}
	if tools[1].Name != "aws" || len(tools[1].Command) != 0 {
		t.Fatalf("second tool = %#v", tools[1])
	}
}
func TestLoadDecodesTargetPreflights(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "deploy-staging" {
  preflights = [
    { name = "cloud session", kind = "session", command = ["sh", "-c", "true"], fix = "Refresh the session." },
  ]
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	preflights := project.Targets["shell/deploy-staging"].Spec().Runtime.Preflights
	if len(preflights) != 1 {
		t.Fatalf("preflights = %#v, want 1 entry", preflights)
	}
	if preflights[0].Name != "cloud session" || preflights[0].Kind != "session" ||
		strings.Join(preflights[0].Command, " ") != "sh -c true" ||
		preflights[0].Fix != "Refresh the session." {
		t.Fatalf("preflight = %#v", preflights[0])
	}
}
func TestLoadDecodesTargetTimeoutAndRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "flaky" {
  timeout = "5m"
  retry {
    attempts                      = 3
    backoff                       = "2s"
    retry_on_quality_gate_failure = true
  }
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	runtime := project.Targets["shell/flaky"].Spec().Runtime
	if runtime.Timeout != 5*time.Minute {
		t.Fatalf("timeout = %s, want 5m", runtime.Timeout)
	}
	if runtime.Retry.Attempts != 3 || runtime.Retry.Backoff != 2*time.Second ||
		!runtime.Retry.RetryOnQualityGateFailure {
		t.Fatalf("retry = %#v, want attempts=3 backoff=2s retry_on_quality", runtime.Retry)
	}
}
func TestLoadRejectsInvalidTargetTimeoutAndRetry(t *testing.T) {
	tests := map[string]string{
		"timeout": `project "example" {}

shell "bad" {
  timeout = "later"
  command = ["true"]
}
`,
		"attempts": `project "example" {}

shell "bad" {
  retry { attempts = 0 }
  command = ["true"]
}
`,
		"backoff": `project "example" {}

shell "bad" {
  retry { attempts = 2 backoff = "later" }
  command = ["true"]
}
`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "Bachfile")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("expected invalid runtime policy error")
			}
		})
	}
}
func TestLoadRejectsPreflightWithoutCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "deploy-staging" {
  preflights = [{ name = "cloud session" }]
  command    = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil ||
		!strings.Contains(
			err.Error(),
			`target "shell/deploy-staging" preflight "cloud session" must set command`,
		) {
		t.Fatalf("error = %v, want missing preflight command validation", err)
	}
}
func TestLoadRejectsUnnamedTargetTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "deploy-staging" {
  tools   = [{ command = ["kubectl", "version", "--client"] }]
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil ||
		!strings.Contains(
			err.Error(),
			`target "shell/deploy-staging" tool requirement must set name`,
		) {
		t.Fatalf("error = %v, want unnamed tool validation", err)
	}
}
func TestLoadDecodesCompletionContracts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "release" {
  command = ["true"]

  success_when {
    output_contains = "Release complete"
  }

  success_when {
    file_exists = "dist/app"
  }

  fail_when {
    output_contains = "ROLLBACK"
  }
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	target := project.Targets["shell/release"]
	if got := len(target.SuccessWhen); got != 2 {
		t.Fatalf("success_when count = %d, want 2", got)
	}
	if got := len(target.FailWhen); got != 1 {
		t.Fatalf("fail_when count = %d, want 1", got)
	}
}
func TestLoadRejectsAmbiguousCompletionContract(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "release" {
  command = ["true"]

  success_when {
    output_contains = "Release complete"
    file_exists     = "dist/app"
  }
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected ambiguous completion contract error")
	}
}
func TestLoadDecodesTargetLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile.hcl")
	contents := `project "example" {}

shell "test-db" {
  lock    = "postgres"
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := project.Targets["shell/test-db"].Lock; got != "postgres" {
		t.Fatalf("lock = %q, want postgres", got)
	}
}
func TestLoadRejectsMissingDependency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Buildfile.hcl")
	contents := `project "example" {}

shell "build" {
  depends_on = ["missing"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected missing dependency error")
	}
}
func TestLoadDecodesNamedInputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Buildfile.hcl")
	contents := `project "example" {}

input "file" "worker" {
  src = "packages/workflows/src"
}

input "file" "shared" {
  srcs = ["package.json", "bun.lock"]
}

shell "test-worker" {
  inputs  = [input.file.worker, input.file.shared]
  command = ["bun", "--filter", "@app/workflows", "test"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := project.resolveInputs(
		project.Targets["shell/test-worker"].Inputs,
	); len(got) != 3 || got[0] != "packages/workflows/src" || got[1] != "package.json" ||
		got[2] != "bun.lock" {
		t.Fatalf("resolved inputs = %v", got)
	}
}
func TestLoadMergesPluginContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Buildfile.hcl")
	contents := `project "example" {}

shell "generated" {
  command = ["true"]
}

shell "build" {
  command = ["true"]
}

plugin "context" {
  shell = "printf '%s' '{\"targets\":{\"shell.build\":{\"depends_on\":[\"shell.generated\"],\"inputs\":[\"generated.ts\"]}}}'"
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	target := project.Targets["shell/build"]
	if len(target.DependsOn) != 1 || target.DependsOn[0] != "shell/generated" {
		t.Fatalf("depends_on = %v, want [shell/generated]", target.DependsOn)
	}
	if len(target.Inputs) != 1 || target.Inputs[0] != "generated.ts" {
		t.Fatalf("inputs = %v, want [generated.ts]", target.Inputs)
	}
}
func TestPluginInputsAndProducedInputsCreateImplicitDependencies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Buildfile.hcl")
	contents := `project "example" {}

resource "workspace_deps" {}

plugin "ts_imports" {
  shell = "printf '%s' '{\"inputs\":{\"api_tests\":[\"packages/api/src/main.ts\"]}}'"
  sources = {
    api_tests = ["packages/api/tests/**/*.test.ts"]
  }
}

shell "install" {
  command  = ["true"]
  produces = [resource.workspace_deps]
}

shell "test-api" {
  command = ["true"]
  inputs  = [resource.workspace_deps, plugin.ts_imports.api_tests]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	target := project.Targets["shell/test-api"]
	if len(target.Inputs) != 2 || target.Inputs[1] != "plugin/ts_imports/api_tests" {
		t.Fatalf("inputs = %v, want plugin input reference", target.Inputs)
	}
	if got := project.resolveInputs(
		[]string{"plugin/ts_imports/api_tests"},
	); len(got) != 1 ||
		got[0] != "packages/api/src/main.ts" {
		t.Fatalf("resolved plugin input = %v", got)
	}
	if got := project.resolveInputs([]string{"resource/workspace_deps"}); len(got) != 0 {
		t.Fatalf("resolved resource input = %v, want no file paths", got)
	}
	if len(target.DependsOn) != 1 || target.DependsOn[0] != "shell/install" {
		t.Fatalf("implicit deps = %v, want [shell/install]", target.DependsOn)
	}
}
