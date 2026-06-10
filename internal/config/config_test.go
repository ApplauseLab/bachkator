package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDecodesTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Buildfile.hcl")
	contents := `project "example" {
  default = "shell.build"
}

shell "install" {
  command = ["bun", "install"]
}

shell "build" {
  depends_on = [shell.install]
  quiet      = true
  shell      = "bun run build"
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if project.DefaultTarget != "shell/build" {
		t.Fatalf("default target = %q, want shell/build", project.DefaultTarget)
	}
	if project.Root != dir {
		t.Fatalf("root = %q, want %q", project.Root, dir)
	}
	if got := project.Targets["shell/build"].DependsOn; len(got) != 1 || got[0] != "shell/install" {
		t.Fatalf("build deps = %v, want [shell/install]", got)
	}
	if !project.Targets["shell/build"].Quiet {
		t.Fatal("build quiet = false, want true")
	}
}

func TestLoadAcceptsStringDotTargetReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {
  default = "shell.build"
}

shell "install" {
  command = ["true"]
}

shell "build" {
  depends_on = ["shell.install"]
  command    = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if project.DefaultTarget != "shell/build" {
		t.Fatalf("default target = %q, want shell/build", project.DefaultTarget)
	}
	if got := project.Targets["shell/build"].DependsOn; len(got) != 1 || got[0] != "shell/install" {
		t.Fatalf("depends_on = %v, want [shell/install]", got)
	}
}

func TestLoadRejectsSlashTargetReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {
  default = "shell/build"
}

shell "build" {
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil ||
		!strings.Contains(err.Error(), `obsolete target reference "shell/build": use type.name`) {
		t.Fatalf("error = %v, want slash reference migration guidance", err)
	}
}

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

func TestLoadDecodesTopLevelQualityConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "test" {
  command = ["bun", "test"]
  outputs = {
    junit = "reports/junit.xml"
    lcov  = "coverage/lcov.info"
  }
}

quality "shell.test" {
  junit {
    path = shell.test.outputs.junit
  }

  cov {
    path = shell.test.outputs.lcov
  }

  quality_gate {
    metric = "tests.failed"
    max    = 0
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
	quality := project.Targets["shell/test"].Spec().Quality
	if len(quality.Reports) != 2 || quality.Reports[0].Format != "junit-xml" ||
		quality.Reports[1].Format != "lcov" {
		t.Fatalf("reports = %#v", quality.Reports)
	}
	if project.Targets["shell/test"].Spec().Cache.NamedOutputs["junit"] != "reports/junit.xml" {
		t.Fatalf("named outputs = %#v", project.Targets["shell/test"].Spec().Cache.NamedOutputs)
	}
	if len(quality.Gates) != 1 || quality.Gates[0].Metric != "tests.failed" ||
		quality.Gates[0].Max == nil ||
		*quality.Gates[0].Max != 0 {
		t.Fatalf("gates = %#v", quality.Gates)
	}
}

func TestLoadDecodesQualityPluginParser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

plugin "lint_parser" {
  type    = "quality"
  command = ["node", "parse-lint.js"]
  timeout = "7s"
  env     = ["MODE=strict"]
}

shell "lint" {
  command = ["true"]
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.json"
    parser = plugin.lint_parser
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
	reports := project.Targets["shell/lint"].Spec().Quality.Reports
	if len(reports) != 1 || reports[0].Parser != "lint_parser" || reports[0].Format != "" {
		t.Fatalf("reports = %#v", reports)
	}
	plugin := project.Plugins["lint_parser"]
	if plugin == nil || plugin.Type != "quality" || plugin.TimeoutDuration != 7*time.Second {
		t.Fatalf("plugin = %#v", plugin)
	}
}

func TestLoadRejectsQualityParserGraphPlugin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

plugin "graph_parser" {
  command = ["node", "graph.js"]
}

shell "lint" {
  command = ["true"]
}

quality "shell.lint" {
  lint {
    path   = ".bach/artifacts/lint.json"
    parser = plugin.graph_parser
  }
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil ||
		!strings.Contains(err.Error(), `parser plugin "graph_parser" must have type = "quality"`) {
		t.Fatalf("error = %v, want non-quality parser plugin validation", err)
	}
}

func TestLoadRejectsQualityForUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

quality "shell.missing" {
  reports = [{ kind = "tests", format = "junit-xml", path = "reports/junit.xml" }]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil ||
		!strings.Contains(err.Error(), `quality block references unknown target "shell/missing"`) {
		t.Fatalf("error = %v, want unknown target validation", err)
	}
}

func TestLoadRejectsInvalidTargetCost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "test" {
  cost    = "tiny"
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid cost error")
	}
	if got := err.Error(); !strings.Contains(got, `target "shell/test" has invalid cost "tiny"`) {
		t.Fatalf("error = %q", got)
	}
}

func TestLoadDecodesTargetAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {
  default = "old-build"
}

alias "old-build" {
  target      = "shell.build"
  deprecated = "Use shell/build."
}

shell "build" {
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
	if project.DefaultTarget != "shell/build" {
		t.Fatalf("default target = %q, want shell/build", project.DefaultTarget)
	}
	alias := project.Aliases["old-build"]
	if alias == nil || alias.Target != "shell/build" || alias.Deprecated != "Use shell/build." {
		t.Fatalf("alias = %#v", alias)
	}
}

func TestLoadRejectsAliasToAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

alias "old-build" {
  target = "older-build"
}

alias "older-build" {
  target = "shell.build"
}

shell "build" {
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected alias chain error")
	}
	if got := err.Error(); !strings.Contains(
		got,
		`alias "old-build" points to alias "older-build"; alias chains are not supported`,
	) {
		t.Fatalf("error = %q", got)
	}
}

func TestLoadRejectsAliasToUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

alias "old-build" {
  target = "shell.missing"
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unknown alias target error")
	}
	if got := err.Error(); !strings.Contains(
		got,
		`alias "old-build" points to unknown target "shell/missing"`,
	) {
		t.Fatalf("error = %q", got)
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

func TestLoadDecodesVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var "github_repo" {
  default = "applause/bachkator"
}

shell "release" {
  command = ["gh", "release", "create", "v0.1.0", "--repo", var.github_repo]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	command := project.Targets["shell/release"].Command
	if got := command[len(command)-1]; got != "applause/bachkator" {
		t.Fatalf("repo = %q, want applause/bachkator", got)
	}
}

func TestLoadVariableOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var "github_repo" {
  default = "applause/bachkator"
}

shell "release" {
  command = ["gh", "release", "create", "v0.1.0", "--repo", var.github_repo]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := LoadWithOptions(
		path,
		LoadOptions{Variables: map[string]string{"github_repo": "example/repo"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	command := project.Targets["shell/release"].Command
	if got := command[len(command)-1]; got != "example/repo" {
		t.Fatalf("repo = %q, want example/repo", got)
	}
}

func TestLoadVariableEnvFallback(t *testing.T) {
	t.Setenv("BACH_VAR_github_repo", "env/repo")
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var "github_repo" {}

shell "release" {
  command = ["gh", "release", "create", "v0.1.0", "--repo", var.github_repo]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	command := project.Targets["shell/release"].Command
	if got := command[len(command)-1]; got != "env/repo" {
		t.Fatalf("repo = %q, want env/repo", got)
	}
}

func TestLoadDecodesProjectEnvAndInterpolatedVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var bla {}

var "foo" {
  default = "foo"
}

var foobar {
  default = "${var.foo}bar"
}

env {
  ENV_2 = "${ENV_1} b ${var.bla} ${var.foobar}"
  ENV_1 = "b"
}

shell "show" {
  command = ["printf", ENV_2]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := LoadWithOptions(path, LoadOptions{Variables: map[string]string{"bla": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := project.Variables["foobar"], "foobar"; got != want {
		t.Fatalf("foobar = %q, want %q", got, want)
	}
	wantEnv := []string{"ENV_1=b", "ENV_2=b b x foobar"}
	if len(project.Env) != len(wantEnv) {
		t.Fatalf("project env = %v, want %v", project.Env, wantEnv)
	}
	for index, want := range wantEnv {
		if project.Env[index] != want {
			t.Fatalf("project env = %v, want %v", project.Env, wantEnv)
		}
	}
	if got := project.Targets["shell/show"].Command[1]; got != "b b x foobar" {
		t.Fatalf("command env ref = %q, want interpolated ENV_2", got)
	}
}

func TestLoadDecodesComputedVariables(t *testing.T) {
	dir := initGitRepo(t)
	writeFile(t, dir, "package.json", `{"name":"example"}`)
	writeFile(t, dir, "Bachfile", `project "example" {}

var "image_tag" {
  default = "${git_short_sha()}${git_dirty_suffix()}"
}

var "deps_tag" {
  default = "deps-${file_hash("package.json")}"
}

env {
  IMAGE_TAG = var.image_tag
}

shell "release" {
  env {
    DEPS_TAG = var.deps_tag
  }
  command = ["printf", var.image_tag, IMAGE_TAG, var.deps_tag]
}

image "app" {
  image = "example/app"
  tags  = [var.image_tag]
}
`)
	runGitOutput(t, dir, "add", "Bachfile", "package.json")
	runGitOutput(t, dir, "commit", "-m", "initial")
	writeFile(t, dir, "dirty.txt", "dirty\n")
	commit := runGitOutput(t, dir, "rev-parse", "HEAD")
	shortSHA := commit[:12]

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := project.Variables["image_tag"], shortSHA+"-dirty"; got != want {
		t.Fatalf("image_tag = %q, want %q", got, want)
	}
	if got := project.Variables["deps_tag"]; len(got) != len("deps-")+12 ||
		got[:len("deps-")] != "deps-" {
		t.Fatalf("deps_tag = %q, want deps- plus 12-char hash", got)
	}
	command := project.Targets["shell/release"].Command
	if command[1] != shortSHA+"-dirty" || command[2] != shortSHA+"-dirty" {
		t.Fatalf("command = %v, want computed image tag in args", command)
	}
	if got := project.Targets["shell/release"].Env[0]; got != "DEPS_TAG="+project.Variables["deps_tag"] {
		t.Fatalf("target env = %v, want deps tag", project.Targets["shell/release"].Env)
	}
	if got, want := project.Targets["image/app"].Tags[0], shortSHA+"-dirty"; got != want {
		t.Fatalf("image tag = %q, want %q", got, want)
	}
}

func TestLoadDecodesTargetEnvBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

env {
  BASE = "base"
}

shell "show" {
  env {
    A = "${BASE}-a"
    B = "${A}-b"
  }
  command = ["sh", "-c", "printf $B"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A=base-a", "B=base-a-b"}
	got := project.Targets["shell/show"].Env
	if len(got) != len(want) {
		t.Fatalf("target env = %v, want %v", got, want)
	}
	for index, value := range want {
		if got[index] != value {
			t.Fatalf("target env = %v, want %v", got, want)
		}
	}
	shell, ok := project.Targets["shell/show"].Spec().Body.(ShellSpec)
	if !ok {
		t.Fatalf("body = %T, want ShellSpec", project.Targets["shell/show"].Spec().Body)
	}
	if got := strings.Join(shell.Command, " "); got != "sh -c printf $B" {
		t.Fatalf("operation = %q, want decoded command", got)
	}
}

func TestLoadAppliesSelectedProfilesBetweenProjectAndTargetEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

env {
  NAMESPACE = "base"
  HOST      = "base.example.com"
}

profile "staging" {
  env {
    NAMESPACE = "staging"
    HOST      = "${NAMESPACE}.example.com"
  }
}

profile "kristiyan" {
  env {
    NAMESPACE = "staging-kristiyan"
  }
}

shell "show" {
  env {
    URL = "https://${HOST}/${NAMESPACE}"
  }
  shell = "printf '$URL'"
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := LoadWithOptions(path, LoadOptions{Profiles: []string{"staging", "kristiyan"}})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := project.SelectedProfiles, []string{
		"staging",
		"kristiyan",
	}; len(got) != len(want) || got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("selected profiles = %v, want %v", got, want)
	}
	if got, want := project.ProfileEnv, []string{
		"HOST=staging.example.com",
		"NAMESPACE=staging-kristiyan",
	}; len(got) != len(want) || got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("profile env = %v, want %v", got, want)
	}
	if got, want := project.Targets["shell/show"].Env, []string{
		"URL=https://staging.example.com/staging-kristiyan",
	}; len(got) != len(want) ||
		got[0] != want[0] {
		t.Fatalf("target env = %v, want %v", got, want)
	}
	shell, ok := project.Targets["shell/show"].Spec().Body.(ShellSpec)
	if !ok {
		t.Fatalf("body = %T, want ShellSpec", project.Targets["shell/show"].Spec().Body)
	}
	if got := shell.Shell; got != "printf '$URL'" {
		t.Fatalf("operation = %q, want decoded shell", got)
	}
}

func TestLoadRejectsUnknownSelectedProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "show" {
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadWithOptions(path, LoadOptions{Profiles: []string{"missing"}}); err == nil {
		t.Fatal("expected unknown profile error")
	}
}
