package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadOnlyHelpers(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	commit := gitTestOutput(t, repo, "rev-parse", "HEAD")

	head, err := Head(ctx, repo)
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	if head != commit {
		t.Fatalf("Head() = %q, want %q", head, commit)
	}

	branch, err := Branch(ctx, repo)
	if err != nil {
		t.Fatalf("Branch() error = %v", err)
	}
	if branch != "main" {
		t.Fatalf("Branch() = %q, want main", branch)
	}

	dirty, err := Dirty(ctx, repo)
	if err != nil {
		t.Fatalf("Dirty() clean repo error = %v", err)
	}
	if dirty {
		t.Fatalf("Dirty() clean repo = true, want false")
	}

	if err := os.WriteFile(
		filepath.Join(repo, "file.txt"),
		[]byte("changed\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	status, err := StatusPorcelain(ctx, repo)
	if err != nil {
		t.Fatalf("StatusPorcelain() error = %v", err)
	}
	if status == "" {
		t.Fatalf("StatusPorcelain() = empty, want dirty status")
	}
	dirty, err = Dirty(ctx, repo)
	if err != nil {
		t.Fatalf("Dirty() dirty repo error = %v", err)
	}
	if !dirty {
		t.Fatalf("Dirty() dirty repo = false, want true")
	}
}
