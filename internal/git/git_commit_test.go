package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCommitHelpers(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	base := gitTestOutput(t, repo, "rev-parse", "HEAD")

	if err := CommitExists(ctx, repo, base); err != nil {
		t.Fatalf("CommitExists() existing commit error = %v", err)
	}
	if err := CommitExists(ctx, repo, "0000000000000000000000000000000000000000"); err == nil {
		t.Fatalf("CommitExists() missing commit error = nil, want error")
	}
	if err := CommitExists(ctx, repo, "--help"); err == nil {
		t.Fatalf("CommitExists() option-like revision error = nil, want error")
	}
	resolved, err := ResolveCommit(ctx, repo, base[:12])
	if err != nil {
		t.Fatalf("ResolveCommit() error = %v", err)
	}
	if resolved != base {
		t.Fatalf("ResolveCommit() = %q, want %q", resolved, base)
	}

	if err := os.WriteFile(
		filepath.Join(repo, "second.txt"),
		[]byte("second\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	gitTestOutput(t, repo, "add", "second.txt")
	gitTestOutput(t, repo, "commit", "-m", "second")
	head := gitTestOutput(t, repo, "rev-parse", "HEAD")

	if err := IsAncestor(ctx, repo, base, head); err != nil {
		t.Fatalf("IsAncestor() ancestor error = %v", err)
	}
	if err := IsAncestor(ctx, repo, head, base); err == nil {
		t.Fatalf("IsAncestor() non-ancestor error = nil, want error")
	}
}

func TestGitDirs(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)

	gitDir, err := GitDir(ctx, repo)
	if err != nil {
		t.Fatalf("GitDir() error = %v", err)
	}
	if gitDir != ".git" {
		t.Fatalf("GitDir() = %q, want .git", gitDir)
	}
	commonDir, err := GitCommonDir(ctx, repo)
	if err != nil {
		t.Fatalf("GitCommonDir() error = %v", err)
	}
	if commonDir != ".git" {
		t.Fatalf("GitCommonDir() = %q, want .git", commonDir)
	}
}
