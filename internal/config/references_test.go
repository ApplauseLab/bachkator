package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
