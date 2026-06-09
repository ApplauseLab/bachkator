package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAffectedPrintsTargetsForExplicitPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

input "file" "api" {
  src = "packages/api/src"
}

shell "lint" {
  command = ["true"]
  inputs  = ["docs"]
}

shell "test-api" {
  command = ["true"]
  inputs  = [input.file.api]
}
`)

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "affected", "packages/api/src/server.go"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	want := "shell/test-api 1 packages/api/src\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestAffectedUsesGitChangedFilesWhenNoPathsProvided(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "test-docs" {
  command = ["true"]
  inputs  = ["docs"]
}
`)
	writeFile(t, filepath.Join(dir, "docs", "guide.md"), "initial\n")
	runGit(t, dir, "init")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	writeFile(t, filepath.Join(dir, "docs", "guide.md"), "changed\n")

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "affected"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	want := "shell/test-docs 1 docs\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestVerboseStreamsQuietTargetOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {
  default = "shell.hello"
}

shell "hello" {
  quiet = true
  shell  = "printf stdout; printf stderr >&2"
}
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "-verbose", "run"}
	if err := Execute(context.Background(), args, &stdout, &stderr, "test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "stdout") {
		t.Fatalf("stdout = %q, want command output", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stderr") {
		t.Fatalf("stderr = %q, want command output", stderr.String())
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
