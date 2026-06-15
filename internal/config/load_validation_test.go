package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsLegacyProjectState(t *testing.T) {
	t.Parallel()

	for _, statePath := range []string{"/tmp/bach-state.db", "../state.db", ".bach/state.db"} {
		t.Run(statePath, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {
  state = "`+statePath+`"
}
`)

			_, err := Load(filepath.Join(dir, "Bachfile"))
			if err == nil ||
				!strings.Contains(err.Error(), "project state is no longer supported") {
				t.Fatalf("Load() error = %v, want legacy state error", err)
			}
		})
	}
}

func TestLoadAcceptsExplicitBackend(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {
  backend {
    type = "stdio"
    command = ["bach", "backend", "sqlite"]
    config = { path = "custom/state.db" }
  }
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.ToSlash(project.Backend.Config["path"]); got != "custom/state.db" {
		t.Fatalf("backend path = %q", got)
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
