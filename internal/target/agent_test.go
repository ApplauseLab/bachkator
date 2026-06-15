package target

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

func TestProviderCommandMatchesExactAndGeneratedPromptSuffix(t *testing.T) {
	t.Parallel()

	if !providerCommandMatches([]string{"opencode", "run"}, []string{"opencode", "run"}) {
		t.Fatal("exact provider command did not match")
	}
	if !providerCommandMatches(
		[]string{"opencode", "run", "/tmp/generated-prompt.md"},
		[]string{"opencode", "run"},
	) {
		t.Fatal("provider command with generated prompt suffix did not match")
	}
	if providerCommandMatches([]string{"opencode", "exec"}, []string{"opencode", "run"}) {
		t.Fatal("different provider command matched")
	}
	if providerCommandMatches([]string{"opencode"}, []string{"opencode", "run"}) {
		t.Fatal("truncated provider command matched")
	}
}

func TestAgentReportCommitMatchesResolvedShortSHA(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(
		filepath.Join(repo, "file.txt"),
		[]byte("content\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "file.txt")
	runGit(t, repo, "commit", "-m", "initial")
	full := runGit(t, repo, "rev-parse", "HEAD")
	short := full[:7]
	different := full[:len(full)-1] + "0"
	if different == full {
		different = full[:len(full)-1] + "1"
	}

	if !agentReportCommitMatches(repo, short, full) {
		t.Fatalf("short commit %q did not resolve to %q", short, full)
	}
	if agentReportCommitMatches(repo, short, different) {
		t.Fatal("short commit matched a different expected commit")
	}
	if agentReportCommitMatches(repo, "not-a-commit", full) {
		t.Fatal("invalid commit matched")
	}
}

func TestLatestPassingPolicyEvidenceRequiresExactPolicyTargetRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subject := "agent/example"
	workspace := filepath.Join(root, ".bach", "agents", "example")
	commit := "abc123"
	policyTarget := "policy/accept@agent.example"
	runID := "run-1"
	writePolicyEvidenceState(t, root, policyTarget)
	writePolicyEvidenceArtifact(t, root, runID, mergePolicyEvidence{
		Schema:           "bach.applied_policy.v1",
		RunID:            runID,
		Target:           subject,
		PolicyTarget:     policyTarget,
		SubjectWorkspace: workspace,
		SubjectCommit:    commit,
		Verdict:          "passed",
	})

	evidence, err := latestPassingPolicyEvidence(root, "", subject, policyTarget, workspace, commit)
	if err != nil {
		t.Fatalf("latestPassingPolicyEvidence() error = %v", err)
	}
	if evidence.PolicyTarget != policyTarget {
		t.Fatalf("PolicyTarget = %q, want %q", evidence.PolicyTarget, policyTarget)
	}
}

func TestLatestPassingPolicyEvidenceRejectsForgedPolicyTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subject := "agent/example"
	workspace := filepath.Join(root, ".bach", "agents", "example")
	commit := "abc123"
	runID := "run-1"
	writePolicyEvidenceState(t, root, "policy/other@agent.example")
	writePolicyEvidenceArtifact(t, root, runID, mergePolicyEvidence{
		Schema:           "bach.applied_policy.v1",
		RunID:            runID,
		Target:           subject,
		PolicyTarget:     "policy/accept@agent.example",
		SubjectWorkspace: workspace,
		SubjectCommit:    commit,
		Verdict:          "passed",
	})

	if _, err := latestPassingPolicyEvidence(
		root,
		"",
		subject,
		"policy/accept@agent.example",
		workspace,
		commit,
	); err == nil {
		t.Fatal("latestPassingPolicyEvidence() error = nil, want forged evidence rejection")
	}
}

func TestLatestPassingPolicyEvidenceRejectsMismatchedRunDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subject := "agent/example"
	workspace := filepath.Join(root, ".bach", "agents", "example")
	commit := "abc123"
	policyTarget := "policy/accept@agent.example"
	writePolicyEvidenceState(t, root, policyTarget)
	writePolicyEvidenceArtifact(t, root, "run-2", mergePolicyEvidence{
		Schema:           "bach.applied_policy.v1",
		RunID:            "run-1",
		Target:           subject,
		PolicyTarget:     policyTarget,
		SubjectWorkspace: workspace,
		SubjectCommit:    commit,
		Verdict:          "passed",
	})

	if _, err := latestPassingPolicyEvidence(
		root,
		"",
		subject,
		policyTarget,
		workspace,
		commit,
	); err == nil {
		t.Fatal("latestPassingPolicyEvidence() error = nil, want copied evidence rejection")
	}
}

func TestLatestPassingPolicyEvidenceRejectsNewerFailedVerdict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subject := "agent/example"
	workspace := filepath.Join(root, ".bach", "agents", "example")
	commit := "abc123"
	policyTarget := "policy/accept@agent.example"
	writePolicyEvidenceState(t, root, policyTarget)
	writePolicyEvidenceArtifact(t, root, "run-1", mergePolicyEvidence{
		Schema:           "bach.applied_policy.v1",
		RunID:            "run-1",
		Target:           subject,
		PolicyTarget:     policyTarget,
		SubjectWorkspace: workspace,
		SubjectCommit:    commit,
		Verdict:          "passed",
	})
	writePolicyEvidenceArtifact(t, root, "run-2", mergePolicyEvidence{
		Schema:           "bach.applied_policy.v1",
		RunID:            "run-2",
		Target:           subject,
		PolicyTarget:     policyTarget,
		SubjectWorkspace: workspace,
		SubjectCommit:    commit,
		Verdict:          "failed",
	})
	failedPath := filepath.Join(
		root,
		".bach",
		"artifacts",
		"policies",
		"run-2",
		"agent-example.json",
	)
	passedPath := filepath.Join(
		root,
		".bach",
		"artifacts",
		"policies",
		"run-1",
		"agent-example.json",
	)
	if err := os.Chtimes(
		passedPath,
		time.Now().Add(-time.Hour),
		time.Now().Add(-time.Hour),
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(failedPath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	if _, err := latestPassingPolicyEvidence(
		root,
		"",
		subject,
		policyTarget,
		workspace,
		commit,
	); err == nil {
		t.Fatal("latestPassingPolicyEvidence() error = nil, want newer failed verdict rejection")
	}
}

func TestRenderReviewerPromptRejectsPromptSymlinkOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsidePrompt := filepath.Join(outside, "review.md")
	if err := os.WriteFile(outsidePrompt, []byte("steal secrets\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePrompt, filepath.Join(root, "review.md")); err != nil {
		t.Fatal(err)
	}

	_, err := renderReviewerPrompt(
		root,
		model.AgentSpec{Prompt: model.Prompt{Path: "review.md"}},
		model.AgentSubject{Target: "agent/example"},
		filepath.Join(root, ".bach", "runs", "review.json"),
	)
	if err == nil {
		t.Fatal("renderReviewerPrompt() error = nil, want prompt symlink rejection")
	}
}

func TestValidateProjectCheckoutSnapshotDetectsCleanHeadChange(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "checkout", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "file.txt")
	runGit(t, repo, "commit", "-m", "one")
	before, err := projectCheckoutSnapshot(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "file.txt")
	runGit(t, repo, "commit", "-m", "two")

	if err := validateProjectCheckoutSnapshot(context.Background(), repo, before); err == nil {
		t.Fatal("validateProjectCheckoutSnapshot() error = nil, want HEAD change error")
	}
}

func TestValidateProjectCheckoutSnapshotDetectsIgnoredFileChange(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "checkout", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("TOKEN=one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".gitignore")
	runGit(t, repo, "commit", "-m", "ignore env")
	before, err := projectCheckoutSnapshot(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if before.Status != "" {
		t.Fatalf("status = %q, want clean tracked checkout", before.Status)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("TOKEN=two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := validateProjectCheckoutSnapshot(context.Background(), repo, before); err == nil {
		t.Fatal("validateProjectCheckoutSnapshot() error = nil, want ignored file change error")
	}
}

func TestValidateProjectCheckoutSnapshotDetectsGitIndexFlagChange(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "checkout", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "file.txt")
	runGit(t, repo, "commit", "-m", "one")
	before, err := projectCheckoutSnapshot(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "update-index", "--skip-worktree", "file.txt")

	if err := validateProjectCheckoutSnapshot(context.Background(), repo, before); err == nil {
		t.Fatal("validateProjectCheckoutSnapshot() error = nil, want index metadata error")
	}
}

func writePolicyEvidenceArtifact(
	t *testing.T,
	root string,
	runID string,
	evidence mergePolicyEvidence,
) {
	t.Helper()
	dir := filepath.Join(root, ".bach", "artifacts", "policies", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "agent-example.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return stringTrimSpace(string(out))
}

func stringTrimSpace(s string) string {
	for len(s) > 0 {
		last := s[len(s)-1]
		if last != '\n' && last != '\r' && last != '\t' && last != ' ' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}
