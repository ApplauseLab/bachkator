package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
