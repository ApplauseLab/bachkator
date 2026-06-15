package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerInjectsGitContextEnvironment(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	runGit(t, dir, "init")
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "staged.txt")
	if err := os.WriteFile(
		filepath.Join(dir, "unstaged.txt"),
		[]byte("unstaged"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"git-context": shellTarget(
				"git-context",
				"printf '%s' \"$BACH_GIT_STAGED_FILES\" > staged-env.txt; printf '%s' \"$BACH_GIT_UNTRACKED_FILES\" > untracked-env.txt; printf '%s' \"$BACH_GIT_DIRTY\" > dirty-env.txt",
			),
		},
	}

	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), project, "git-context"); err != nil {
		t.Fatal(err)
	}
	staged, err := os.ReadFile(filepath.Join(dir, "staged-env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(staged) != "staged.txt" {
		t.Fatalf("staged files env = %q, want staged.txt", string(staged))
	}
	untracked, err := os.ReadFile(filepath.Join(dir, "untracked-env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(untracked), "unstaged.txt") {
		t.Fatalf("untracked files env = %q, want unstaged.txt", string(untracked))
	}
	dirty, err := os.ReadFile(filepath.Join(dir, "dirty-env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(dirty) != "1" {
		t.Fatalf("dirty env = %q, want 1", string(dirty))
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %s\n%s", strings.Join(args, " "), err, string(output))
	}
}
