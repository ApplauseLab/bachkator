package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadImportsLocalFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {
  default = "shell.test"
}

import "./bach/go.bach"
`)
	writeTestFile(t, filepath.Join(dir, "bach", "go.bach"), `shell "test" {
  command = ["true"]
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := project.Targets["shell/test"]; !ok {
		t.Fatalf("targets = %#v, want imported shell/test", project.Targets)
	}
}
func TestLoadImportsNestedRelativeToImportingFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}
import "./bach/go.bach"
`)
	writeTestFile(t, filepath.Join(dir, "bach", "go.bach"), `import "./shared/docs.bach"
shell "test" {
  depends_on = [shell.docs]
  command    = ["true"]
}
`)
	writeTestFile(t, filepath.Join(dir, "bach", "shared", "docs.bach"), `shell "docs" {
  command = ["true"]
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	deps := project.Targets["shell/test"].DependsOn
	if len(deps) != 1 || deps[0] != "shell/docs" {
		t.Fatalf("deps = %v, want [shell/docs]", deps)
	}
}
func TestLoadDedupesRepeatedImportWithoutError(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}
import "./bach/go.bach"
import "./bach/go.bach"
`)
	writeTestFile(t, filepath.Join(dir, "bach", "go.bach"), `shell "test" {
  command = ["true"]
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Targets) != 1 {
		t.Fatalf("targets = %#v, want exactly one imported target", project.Targets)
	}
}
func TestLoadImportErrors(t *testing.T) {
	tests := map[string]struct {
		files map[string]string
		want  string
	}{
		"missing": {
			files: map[string]string{
				"Bachfile": `project "example" {}
import "./missing.bach"
`,
			},
			want: `import "./missing.bach"`,
		},
		"cycle": {
			files: map[string]string{
				"Bachfile": `project "example" {}
import "./a.bach"
`,
				"a.bach": `import "./b.bach"
`,
				"b.bach": `import "./a.bach"
`,
			},
			want: "import cycle:",
		},
		"duplicate_target": {
			files: map[string]string{
				"Bachfile": `project "example" {}
import "./go.bach"
shell "test" { command = ["true"] }
`,
				"go.bach": `shell "test" { command = ["true"] }
`,
			},
			want: `duplicate target "shell/test"`,
		},
		"imported_project": {
			files: map[string]string{
				"Bachfile": `project "example" {}
import "./project.bach"
`,
				"project.bach": `project "other" {}
`,
			},
			want: "imported Bachfile must not declare a project block",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range tt.files {
				writeTestFile(t, filepath.Join(dir, name), contents)
			}
			_, err := Load(filepath.Join(dir, "Bachfile"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
func TestLoadImportsAllDeclarationKinds(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {
  default = "legacy-test"
}
import "./pack.bach"
`)
	writeTestFile(t, filepath.Join(dir, "pack.bach"), `var "message" {
  default = "ok"
}

env {
  MESSAGE = var.message
}

profile "ci" {
  env { MODE = "ci" }
}

input "file" "source" {
  src = "input.txt"
}

resource "database" {}

plugin "graph" {
  type = "graph"
}

shell "test" {
  command = ["true"]
  inputs  = ["file/source"]
}

quality "shell.test" {
  quality_gate {
    metric = "tests.failed"
    max    = 0
  }
}

alias "legacy-test" {
  target = "shell.test"
}
`)

	project, err := LoadWithOptions(
		filepath.Join(dir, "Bachfile"),
		LoadOptions{Profiles: []string{"ci"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if project.Variables["message"] != "ok" || project.DefaultTarget != "shell/test" {
		t.Fatalf("project = %#v", project)
	}
	if _, ok := project.Inputs["file/source"]; !ok {
		t.Fatal("missing imported input")
	}
	if _, ok := project.Resources["resource/database"]; !ok {
		t.Fatal("missing imported resource")
	}
	if _, ok := project.Plugins["graph"]; !ok {
		t.Fatal("missing imported plugin")
	}
	if _, ok := project.Aliases["legacy-test"]; !ok {
		t.Fatal("missing imported alias")
	}
	if got := project.Targets["shell/test"].QualityGates; len(got) != 1 {
		t.Fatalf("quality gates = %#v, want 1", got)
	}
	gotEnv := strings.Join(append(project.Env, project.ProfileEnv...), ",")
	if !strings.Contains(gotEnv, "MESSAGE=ok") || !strings.Contains(gotEnv, "MODE=ci") {
		t.Fatalf("env = %q, want imported env and profile", gotEnv)
	}
}
func TestLoadImportsLocalBachfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {
  default = "shell.test"
}

import "./bach/go.bach"
`)
	writeFile(t, dir, "bach/go.bach", `shell "test" {
  command = ["true"]
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := project.Targets["shell/test"]; !ok {
		t.Fatalf("imported target missing: %v", project.Targets)
	}
}
func TestLoadImportsNestedRelativeToImporter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

import "./bach/go.bach"
`)
	writeFile(t, dir, "bach/go.bach", `import "./docs.bach"

shell "test" {
  depends_on = [shell.docs]
  command    = ["true"]
}
`)
	writeFile(t, dir, "bach/docs.bach", `shell "docs" {
  command = ["true"]
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if got := project.Targets["shell/test"].DependsOn; len(got) != 1 || got[0] != "shell/docs" {
		t.Fatalf("depends_on = %v, want [shell/docs]", got)
	}
}
func TestLoadDedupesRepeatedImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

import "./bach/go.bach"
import "./bach/go.bach"
`)
	writeFile(t, dir, "bach/go.bach", `shell "test" {
  command = ["true"]
}
`)

	if _, err := Load(filepath.Join(dir, "Bachfile")); err != nil {
		t.Fatal(err)
	}
}
func TestLoadRejectsMissingImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

import "./missing.bach"
`)

	_, err := Load(filepath.Join(dir, "Bachfile"))
	if err == nil {
		t.Fatal("expected missing import error")
	}
	if !strings.Contains(err.Error(), "Bachfile:3") ||
		!strings.Contains(err.Error(), "missing.bach") {
		t.Fatalf("error = %v, want source path and import path", err)
	}
}
func TestLoadRejectsImportCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

import "./a.bach"
`)
	writeFile(t, dir, "a.bach", `import "./b.bach"
`)
	writeFile(t, dir, "b.bach", `import "./a.bach"
`)

	_, err := Load(filepath.Join(dir, "Bachfile"))
	if err == nil {
		t.Fatal("expected import cycle error")
	}
	if !strings.Contains(err.Error(), "import cycle") ||
		!strings.Contains(err.Error(), "a.bach") ||
		!strings.Contains(err.Error(), "b.bach") {
		t.Fatalf("error = %v, want import cycle path", err)
	}
}
func TestLoadRejectsDuplicateImportedTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

shell "test" {
  command = ["true"]
}

import "./bach/go.bach"
`)
	writeFile(t, dir, "bach/go.bach", `shell "test" {
  command = ["true"]
}
`)

	_, err := Load(filepath.Join(dir, "Bachfile"))
	if err == nil {
		t.Fatal("expected duplicate target error")
	}
	if !strings.Contains(err.Error(), `duplicate target "shell/test"`) {
		t.Fatalf("error = %v, want duplicate target", err)
	}
}
func TestLoadRejectsImportedProjectBlock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

import "./bach/go.bach"
`)
	writeFile(t, dir, "bach/go.bach", `project "imported" {}
`)

	_, err := Load(filepath.Join(dir, "Bachfile"))
	if err == nil {
		t.Fatal("expected imported project error")
	}
	if !strings.Contains(err.Error(), "imported Bachfile must not declare a project block") {
		t.Fatalf("error = %v, want imported project error", err)
	}
}
func TestLoadRejectsNonStringImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

import var.some_path
`)

	_, err := Load(filepath.Join(dir, "Bachfile"))
	if err == nil {
		t.Fatal("expected import syntax error")
	}
	if !strings.Contains(err.Error(), "import path must be a string literal") {
		t.Fatalf("error = %v, want string literal error", err)
	}
}
func TestLoadDoesNotTreatTargetBodyImportTextAsImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

shell "show" {
  shell = <<EOT
import "./not-a-bachfile"
EOT
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := project.Targets["shell/show"]; !ok {
		t.Fatalf("target missing: %v", project.Targets)
	}
}
func TestLoadImportScannerIgnoresBracesInComments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Bachfile", `project "example" {}

# A top-level comment with { should not hide this import.
/* A block comment with } should not affect scanner depth. */
import "./bach/go.bach"

shell "show" {
  # A nested comment with } should not make this line top-level.
  shell = <<EOT
import "./not-a-bachfile"
EOT
}
`)
	writeFile(t, dir, "bach/go.bach", `shell "test" {
  command = ["true"]
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := project.Targets["shell/test"]; !ok {
		t.Fatalf("imported target missing: %v", project.Targets)
	}
	if _, ok := project.Targets["shell/show"]; !ok {
		t.Fatalf("root target missing: %v", project.Targets)
	}
}
