package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitTestOutput(t, repo, "init", "-b", "main")
	gitTestOutput(t, repo, "config", "user.email", "bach@example.test")
	gitTestOutput(t, repo, "config", "user.name", "Bach Test")
	if err := os.WriteFile(
		filepath.Join(repo, "file.txt"),
		[]byte("initial\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	gitTestOutput(t, repo, "add", "file.txt")
	gitTestOutput(t, repo, "commit", "-m", "initial")
	return repo
}

func gitTestOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
