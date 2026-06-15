package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
func TestLoadDecodesQualityRegoPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "scan" {
  command = ["true"]
}

quality "shell.scan" {
  reports = [{ kind = "security", format = "agent-report-v1", path = "security.json" }]

  rego_policy {
    path    = "policies/no-critical.rego"
    package = "bach.policy"
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
	quality := project.Targets["shell/scan"].Spec().Quality
	if len(quality.RegoPolicies) != 1 {
		t.Fatalf("rego policies = %#v", quality.RegoPolicies)
	}
	policy := quality.RegoPolicies[0]
	if policy.Path != "policies/no-critical.rego" || policy.Package != "bach.policy" {
		t.Fatalf("rego policy = %#v", policy)
	}
}
func TestLoadRejectsRegoPolicyWithoutPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "scan" {
  command = ["true"]
}

quality "shell.scan" {
  rego_policy {
    package = "bach.policy"
  }
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil ||
		!strings.Contains(err.Error(), `target "shell/scan" rego_policy must set path`) {
		t.Fatalf("error = %v, want rego path validation", err)
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
