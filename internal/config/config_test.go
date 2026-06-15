package config

import (
	"os"
	"path/filepath"
	"testing"
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
func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
