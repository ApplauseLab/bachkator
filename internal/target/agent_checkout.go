package target

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
)

func samePath(a string, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil {
		a = absA
	}
	if errB == nil {
		b = absB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func validateMergeSubjectWorkspace(
	ctx context.Context,
	workspace string,
	branch string,
	commit string,
) error {
	currentBranch, err := gitpkg.Branch(ctx, workspace)
	if err != nil {
		return fmt.Errorf("subject workspace branch: %w", err)
	}
	if currentBranch != branch {
		return fmt.Errorf("subject workspace branch = %q, want %q", currentBranch, branch)
	}
	currentCommit, err := gitpkg.Head(ctx, workspace)
	if err != nil {
		return fmt.Errorf("subject workspace commit: %w", err)
	}
	if currentCommit != commit {
		return fmt.Errorf("subject workspace commit = %q, want %q", currentCommit, commit)
	}
	status, err := gitpkg.StatusPorcelain(ctx, workspace)
	if err != nil {
		return fmt.Errorf("subject workspace status: %w", err)
	}
	if status != "" {
		return fmt.Errorf("subject workspace %q is dirty", workspace)
	}
	return nil
}

func validateReviewedWorkspaceUnchanged(
	ctx context.Context,
	workspace string,
	subject model.AgentSubject,
) error {
	commit, err := gitpkg.Head(ctx, workspace)
	if err != nil {
		return fmt.Errorf("subject workspace commit after reviewers: %w", err)
	}
	if commit != subject.Commit {
		return fmt.Errorf(
			"reviewers changed subject commit: got %s want %s",
			commit,
			subject.Commit,
		)
	}
	if subject.Branch != "" {
		branch, err := gitpkg.Branch(ctx, workspace)
		if err != nil {
			return fmt.Errorf("subject workspace branch after reviewers: %w", err)
		}
		if branch != subject.Branch {
			return fmt.Errorf(
				"reviewers changed subject branch: got %q want %q",
				branch,
				subject.Branch,
			)
		}
	}
	status, err := gitStatusExcludingBach(ctx, workspace)
	if err != nil {
		return fmt.Errorf("subject workspace status after reviewers: %w", err)
	}
	if status != "" {
		return fmt.Errorf("reviewers left subject workspace dirty")
	}
	return nil
}

type projectCheckoutState struct {
	Branch      string
	Head        string
	Status      string
	GitMetadata string
	Ignored     string
}

func projectCheckoutSnapshot(ctx context.Context, root string) (projectCheckoutState, error) {
	branch, err := gitpkg.Branch(ctx, root)
	if err != nil {
		return projectCheckoutState{}, fmt.Errorf("project checkout branch: %w", err)
	}
	head, err := gitpkg.Head(ctx, root)
	if err != nil {
		return projectCheckoutState{}, fmt.Errorf("project checkout HEAD: %w", err)
	}
	status, err := gitStatusExcludingBach(ctx, root)
	if err != nil {
		return projectCheckoutState{}, fmt.Errorf("project checkout status: %w", err)
	}
	gitMetadata, err := gitMetadataSnapshot(ctx, root)
	if err != nil {
		return projectCheckoutState{}, err
	}
	ignored, err := ignoredFilesSnapshot(ctx, root)
	if err != nil {
		return projectCheckoutState{}, err
	}
	return projectCheckoutState{
		Branch: branch, Head: head, Status: status, GitMetadata: gitMetadata, Ignored: ignored,
	}, nil
}

func validateProjectCheckoutSnapshot(
	ctx context.Context,
	root string,
	before projectCheckoutState,
) error {
	after, err := projectCheckoutSnapshot(ctx, root)
	if err != nil {
		return err
	}
	if after.Branch != before.Branch {
		return fmt.Errorf(
			"provider changed main checkout branch: got %q want %q",
			after.Branch,
			before.Branch,
		)
	}
	if after.Head != before.Head {
		return fmt.Errorf(
			"provider changed main checkout HEAD: got %s want %s",
			after.Head,
			before.Head,
		)
	}
	if after.Status != before.Status {
		return fmt.Errorf("provider changed main checkout status: %s", after.Status)
	}
	if after.GitMetadata != before.GitMetadata {
		return fmt.Errorf("provider changed main checkout git metadata")
	}
	if after.Ignored != before.Ignored {
		return fmt.Errorf("provider changed ignored main checkout files")
	}
	return nil
}

func gitMetadataSnapshot(ctx context.Context, root string) (string, error) {
	hash := sha256.New()
	gitDir, err := gitpkg.GitDir(ctx, root)
	if err != nil {
		return "", fmt.Errorf("project git dir: %w", err)
	}
	commonDir, err := gitpkg.GitCommonDir(ctx, root)
	if err != nil {
		return "", fmt.Errorf("project git common dir: %w", err)
	}
	for _, dir := range []string{gitDir, commonDir} {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		for _, rel := range []string{"HEAD", "config", "packed-refs", "index", "info", "refs", "hooks"} {
			if err := hashEvidencePath(hash, filepath.Join(dir, rel)); err != nil {
				return "", err
			}
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func ignoredFilesSnapshot(ctx context.Context, root string) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		"git",
		"ls-files",
		"--others",
		"--ignored",
		"--exclude-standard",
		"-z",
	)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("project ignored files: %w", err)
	}
	hash := sha256.New()
	for _, rel := range strings.Split(string(output), "\x00") {
		rel = strings.TrimSpace(rel)
		if rel == "" || rel == ".bach" || strings.HasPrefix(rel, ".bach/") {
			continue
		}
		_, _ = fmt.Fprintf(hash, "ignored %s\n", rel)
		if err := hashEvidencePath(hash, filepath.Join(root, rel)); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func gitStatusExcludingBach(ctx context.Context, workspace string) (string, error) {
	status, err := gitpkg.StatusPorcelain(ctx, workspace)
	if err != nil {
		return "", err
	}
	kept := []string{}
	for _, line := range strings.Split(status, "\n") {
		if line == "" {
			continue
		}
		path := strings.TrimSpace(line[2:])
		path = strings.TrimPrefix(path, "\"")
		if strings.HasPrefix(path, ".bach/") || path == ".bach" {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), nil
}
