package git

import (
	"context"
	"os/exec"
	"sort"
	"strings"
)

type Context struct {
	Branch        string
	Commit        string
	Dirty         bool
	StagedFiles   []string
	UnstagedFiles []string
	Untracked     []string
	ChangedFiles  []string
}

func LoadContext(ctx context.Context, root string) Context {
	if _, err := output(ctx, root, "rev-parse", "--show-toplevel"); err != nil {
		return Context{}
	}
	branch, _ := output(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	commit, _ := output(ctx, root, "rev-parse", "HEAD")
	status, _ := output(ctx, root, "status", "--porcelain")
	staged, _ := output(ctx, root, "diff", "--name-only", "--cached")
	unstaged, _ := output(ctx, root, "diff", "--name-only")
	untracked, _ := output(ctx, root, "ls-files", "--others", "--exclude-standard")

	context := Context{
		Branch:        branch,
		Commit:        commit,
		Dirty:         status != "",
		StagedFiles:   splitLines(staged),
		UnstagedFiles: splitLines(unstaged),
		Untracked:     splitLines(untracked),
	}
	context.ChangedFiles = uniqueSorted(
		append(
			append([]string{}, context.StagedFiles...),
			append(context.UnstagedFiles, context.Untracked...)...),
	)
	return context
}

func ChangedFiles(ctx context.Context, root string) []string {
	return LoadContext(ctx, root).ChangedFiles
}

func Head(ctx context.Context, root string) (string, error) {
	return output(ctx, root, "rev-parse", "HEAD")
}

func Branch(ctx context.Context, root string) (string, error) {
	return output(ctx, root, "branch", "--show-current")
}

func StatusPorcelain(ctx context.Context, root string) (string, error) {
	return output(ctx, root, "status", "--porcelain")
}

func Dirty(ctx context.Context, root string) (bool, error) {
	status, err := StatusPorcelain(ctx, root)
	if err != nil {
		return false, err
	}
	return status != "", nil
}

func CommitExists(ctx context.Context, root string, commit string) error {
	_, err := ResolveCommit(ctx, root, commit)
	return err
}

func IsAncestor(ctx context.Context, root string, ancestor string, descendant string) error {
	_, err := output(ctx, root, "merge-base", "--is-ancestor", ancestor, descendant)
	return err
}

func GitDir(ctx context.Context, root string) (string, error) {
	return output(ctx, root, "rev-parse", "--git-dir")
}

func GitCommonDir(ctx context.Context, root string) (string, error) {
	return output(ctx, root, "rev-parse", "--git-common-dir")
}

func ResolveCommit(ctx context.Context, root string, commit string) (string, error) {
	return output(ctx, root, "rev-parse", "--verify", "--end-of-options", commit+"^{commit}")
}

func (g Context) Env() []string {
	dirty := "0"
	dirtySuffix := ""
	if g.Dirty {
		dirty = "1"
		dirtySuffix = "-dirty"
	}
	return []string{
		"BACH_GIT_BRANCH=" + g.Branch,
		"BACH_GIT_COMMIT=" + g.Commit,
		"BACH_GIT_DIRTY=" + dirty,
		"BACH_GIT_DIRTY_SUFFIX=" + dirtySuffix,
		"BACH_GIT_STAGED_FILES=" + strings.Join(g.StagedFiles, "\n"),
		"BACH_GIT_UNSTAGED_FILES=" + strings.Join(g.UnstagedFiles, "\n"),
		"BACH_GIT_UNTRACKED_FILES=" + strings.Join(g.Untracked, "\n"),
		"BACH_GIT_CHANGED_FILES=" + strings.Join(g.ChangedFiles, "\n"),
	}
}

func output(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func splitLines(value string) []string {
	if value == "" {
		return nil
	}
	lines := strings.Split(value, "\n")
	out := lines[:0]
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
