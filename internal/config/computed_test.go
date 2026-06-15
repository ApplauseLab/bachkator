package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestComputedGitShortSHA(t *testing.T) {
	dir := initGitRepo(t)
	writeFile(t, dir, "README.md", "hello\n")
	runGitOutput(t, dir, "add", "README.md")
	runGitOutput(t, dir, "commit", "-m", "initial")
	commit := runGitOutput(t, dir, "rev-parse", "HEAD")

	got, err := computeGitShortSHA(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := commit[:12]; got != want {
		t.Fatalf("short sha = %q, want %q", got, want)
	}
}

func TestComputedGitDirtySuffix(t *testing.T) {
	dir := initGitRepo(t)
	writeFile(t, dir, "README.md", "hello\n")
	runGitOutput(t, dir, "add", "README.md")
	runGitOutput(t, dir, "commit", "-m", "initial")

	got, err := computeGitDirtySuffix(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("clean dirty suffix = %q, want empty", got)
	}

	writeFile(t, dir, "README.md", "changed\n")
	got, err = computeGitDirtySuffix(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "-dirty" {
		t.Fatalf("dirty suffix = %q, want -dirty", got)
	}
}

func TestComputedFileHashDeterministicForDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src/b.txt", "b\n")
	writeFile(t, dir, "src/a.txt", "a\n")
	writeFile(t, dir, "src/nested/c.txt", "c\n")

	got, err := computeFileHash(dir, []string{"src"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := computeFileHash(dir, []string{"src/nested", "src/b.txt", "src/a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if got != again {
		t.Fatalf("directory hash = %q, explicit hash = %q", got, again)
	}
	if len(got) != 12 {
		t.Fatalf("hash length = %d, want 12", len(got))
	}

	writeFile(t, dir, "src/a.txt", "changed\n")
	changed, err := computeFileHash(dir, []string{"src"})
	if err != nil {
		t.Fatal(err)
	}
	if changed == got {
		t.Fatalf("hash did not change after file content update: %q", changed)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitOutput(t, dir, "init")
	runGitOutput(t, dir, "config", "user.email", "bach@example.com")
	runGitOutput(t, dir, "config", "user.name", "Bach Test")
	return dir
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func writeFile(t *testing.T, root string, name string, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
