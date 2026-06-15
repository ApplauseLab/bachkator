package target

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/state"
)

func generatedPolicyTargetName(policyName string, subjectTarget string) string {
	return model.GeneratedPolicyTargetAddress(policyName, subjectTarget).LegacyName()
}

func maxAttempts(body model.AgentSpec) int {
	if body.Improve.MaxAttempts > 0 {
		return body.Improve.MaxAttempts
	}
	return 1
}

func (agentHandler) FingerprintParts(body model.TargetBody) map[string]string {
	agent, _ := body.(model.AgentSpec)
	return map[string]string{
		"mode":             agent.Mode,
		"provider-name":    agent.Provider.Name,
		"provider-type":    agent.Provider.Type,
		"provider-command": strings.Join(providerBaseCommand(agent.Provider), "\x00"),
		"role":             agent.Role,
		"prompt":           agent.Prompt.Path,
		"plan":             agent.Plan,
		"workspace-mode":   agent.Workspace.Mode,
		"workspace-path":   agent.Workspace.Path,
		"git-branch":       agent.Git.Branch,
		"git-commit":       agent.Git.Commit,
		"policy":           agent.Policy.Name,
		"reviewers":        strings.Join(agent.Policy.Reviewers, "\x00"),
		"max-attempts":     strconv.Itoa(agent.Improve.MaxAttempts),
		"until":            agent.Improve.Until,
	}
}

func (agentHandler) CompositeChildren(model.TargetBody) []CompositeChild { return nil }

func (agentHandler) prepareWorkspace(
	ctx context.Context,
	root string,
	workspace string,
	branch string,
	sourceCommit string,
) error {
	if info, err := os.Stat(workspace); err == nil {
		return prepareExistingWorkspace(ctx, workspace, branch, sourceCommit, info)
	}
	if err := os.MkdirAll(filepath.Dir(workspace), 0o755); err != nil {
		return err
	}
	clone := exec.CommandContext(ctx, "git", "clone", root, workspace)
	if output, err := clone.CombinedOutput(); err != nil {
		return fmt.Errorf("agent workspace clone: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := runGitCommand(ctx, workspace, "switch", "-c", branch); err != nil {
		return fmt.Errorf("agent workspace branch %q: %w", branch, err)
	}
	return nil
}

func prepareExistingWorkspace(
	ctx context.Context,
	workspace string,
	branch string,
	sourceCommit string,
	info os.FileInfo,
) error {
	if !info.IsDir() {
		return fmt.Errorf("agent workspace %q exists and is not a directory", workspace)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".git")); err != nil {
		return fmt.Errorf("agent workspace %q is not a git clone", workspace)
	}
	status, err := gitpkg.StatusPorcelain(ctx, workspace)
	if err != nil {
		return fmt.Errorf("agent workspace status: %w", err)
	}
	if status != "" {
		return fmt.Errorf("agent workspace %q is dirty", workspace)
	}
	if err := runGitCommand(ctx, workspace, "switch", branch); err != nil {
		if createErr := runGitCommand(ctx, workspace, "switch", "-c", branch); createErr != nil {
			return fmt.Errorf("agent workspace branch %q: %w", branch, err)
		}
	}
	if err := gitpkg.IsAncestor(ctx, workspace, sourceCommit, "HEAD"); err != nil {
		return fmt.Errorf(
			"agent workspace %q is stale: source commit %s is not an ancestor of HEAD",
			workspace,
			sourceCommit,
		)
	}
	return nil
}

func (agentHandler) validateGitEvidence(
	ctx context.Context,
	workspace string,
	branch string,
	beforeCommit string,
	afterCommit string,
	mode string,
) error {
	currentBranch, err := gitpkg.Branch(ctx, workspace)
	if err != nil {
		return fmt.Errorf("agent workspace branch: %w", err)
	}
	if currentBranch != branch {
		return fmt.Errorf("agent workspace branch = %q, want %q", currentBranch, branch)
	}
	if beforeCommit != afterCommit {
		if err := gitpkg.IsAncestor(ctx, workspace, beforeCommit, afterCommit); err != nil {
			return fmt.Errorf(
				"agent workspace commit %s is not a descendant of %s",
				afterCommit,
				beforeCommit,
			)
		}
	}
	if mode == "plan" {
		return nil
	}
	status, err := gitpkg.StatusPorcelain(ctx, workspace)
	if err != nil {
		return fmt.Errorf("agent workspace status: %w", err)
	}
	if status != "" {
		return fmt.Errorf("agent workspace %q is dirty after provider execution", workspace)
	}
	return nil
}

func workspaceDirty(ctx context.Context, workspace string) bool {
	dirty, err := gitpkg.Dirty(ctx, workspace)
	return err == nil && dirty
}

func runGitCommand(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	_, err := cmd.CombinedOutput()
	return err
}

func copyEnv(env map[string]string) map[string]string {
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[key] = value
	}
	return out
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "reviewer"
	}
	return strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(value)
}

func validateTrustedEvidenceUnchanged(root string, statePath string, before string) error {
	after, err := trustedEvidenceSnapshot(root, statePath)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf(
			"provider changed Bach-owned policy evidence or policy run state",
		)
	}
	return nil
}

func trustedEvidenceSnapshot(root string, statePath string) (string, error) {
	hash := sha256.New()
	paths := []string{
		filepath.Join(root, ".bach", "artifacts", "policies"),
	}
	for _, path := range paths {
		if err := hashEvidencePathWithOptions(hash, path, true); err != nil {
			return "", err
		}
	}
	if err := hashStateEvidence(hash, root, statePath); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashStateEvidence(hash evidenceHasher, root string, statePath string) error {
	store, err := openTrustedEvidenceStore(root, statePath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	snapshot, err := store.Load()
	if err != nil {
		return err
	}
	records := []string{}
	approvals, err := store.ListFactoryApprovalEvidence()
	if err != nil {
		return err
	}
	for _, approval := range approvals {
		records = append(records, fmt.Sprintf(
			"factory-approval %s %s %s %s %s %s %s %s %s %s %s %s\n",
			approval.ID,
			approval.Factory,
			approval.Workflow,
			approval.WorkItemID,
			approval.AttemptID,
			approval.Phase,
			approval.PlanPath,
			approval.PlanHash,
			approval.ApprovedAt.UTC().Format(time.RFC3339Nano),
			approval.Approver,
			approval.ApproverSource,
			approval.Reason,
		))
		metadataKeys := make([]string, 0, len(approval.Metadata))
		for key := range approval.Metadata {
			metadataKeys = append(metadataKeys, key)
		}
		sort.Strings(metadataKeys)
		for _, key := range metadataKeys {
			records = append(records, fmt.Sprintf(
				"factory-approval-metadata %s %s %s\n",
				approval.ID,
				key,
				approval.Metadata[key],
			))
		}
	}
	for target, record := range snapshot.Targets {
		records = append(records, fmt.Sprintf(
			"target-state %s %s %s\n",
			target,
			record.Fingerprint,
			record.CompletedAt.UTC().Format(time.RFC3339Nano),
		))
		partKeys := make([]string, 0, len(record.FingerprintParts))
		for key := range record.FingerprintParts {
			partKeys = append(partKeys, key)
		}
		sort.Strings(partKeys)
		for _, key := range partKeys {
			records = append(records, fmt.Sprintf(
				"target-state-part %s %s %s\n",
				target,
				key,
				record.FingerprintParts[key],
			))
		}
	}
	for _, run := range snapshot.Runs {
		containsPolicyTarget := strings.HasPrefix(run.Target, "policy/")
		for target := range run.Targets {
			if strings.HasPrefix(target, "policy/") {
				containsPolicyTarget = true
				break
			}
		}
		if containsPolicyTarget {
			records = append(records, fmt.Sprintf(
				"run %s %s %s %t %t %s %s %s\n",
				run.ID,
				run.Target,
				run.Status,
				run.DryRun,
				run.Force,
				run.StartedAt.UTC().Format(time.RFC3339Nano),
				run.FinishedAt.UTC().Format(time.RFC3339Nano),
				run.LogDir,
			))
		}
		for target, targetRun := range run.Targets {
			if !containsPolicyTarget {
				continue
			}
			exitCode := ""
			if targetRun.ExitCode != nil {
				exitCode = strconv.Itoa(*targetRun.ExitCode)
			}
			records = append(records, fmt.Sprintf(
				"target %s %s %s %s %s %s %s %s %s\n",
				run.ID,
				run.Target,
				target,
				targetRun.Status,
				targetRun.StartedAt.UTC().Format(time.RFC3339Nano),
				targetRun.FinishedAt.UTC().Format(time.RFC3339Nano),
				targetRun.LogPath,
				targetRun.Operation,
				exitCode,
			))
		}
		if containsPolicyTarget {
			for _, artifact := range run.Artifacts {
				records = append(records, fmt.Sprintf(
					"artifact %s %s %s %s %s %s\n",
					artifact.RunID,
					artifact.Target,
					artifact.Kind,
					artifact.Path,
					artifact.Value,
					artifact.CreatedAt.UTC().Format(time.RFC3339Nano),
				))
			}
		}
	}
	ledgers, err := store.ListPlanLedgerEvidence()
	if err != nil {
		return err
	}
	for _, ledger := range ledgers {
		records = append(records, fmt.Sprintf(
			"plan-ledger %s %s %s %s %s %s %s %s\n",
			ledger.LedgerID,
			ledger.PlanID,
			ledger.Status,
			ledger.Hash,
			ledger.RunID,
			ledger.Commit,
			ledger.RecordedAt.UTC().Format(time.RFC3339Nano),
			ledger.ImplementedAt.UTC().Format(time.RFC3339Nano),
		))
		for _, evidence := range ledger.Evidence {
			content, err := json.Marshal(evidence.Content)
			if err != nil {
				return err
			}
			records = append(records, fmt.Sprintf(
				"plan-evidence %s %s %s %s %s\n",
				ledger.LedgerID,
				evidence.ID,
				evidence.Kind,
				evidence.Hash,
				string(content),
			))
			metadataKeys := make([]string, 0, len(evidence.Metadata))
			for key := range evidence.Metadata {
				metadataKeys = append(metadataKeys, key)
			}
			sort.Strings(metadataKeys)
			for _, key := range metadataKeys {
				records = append(records, fmt.Sprintf(
					"plan-evidence-metadata %s %s %s %s\n",
					ledger.LedgerID,
					evidence.ID,
					key,
					evidence.Metadata[key],
				))
			}
		}
	}
	sort.Strings(records)
	_, _ = hash.Write([]byte("policy-runs\n"))
	for _, record := range records {
		_, _ = hash.Write([]byte(record))
	}
	return nil
}

func openTrustedEvidenceStore(root string, statePath string) (*state.Store, error) {
	if statePath == "" {
		statePath = filepath.Join(root, ".bach", "state.db")
	} else if !filepath.IsAbs(statePath) {
		statePath = filepath.Join(root, statePath)
	}
	return state.OpenReadOnlyStore(statePath)
}

func hashEvidencePath(hash interface{ Write([]byte) (int, error) }, path string) error {
	return hashEvidencePathWithOptions(hash, path, false)
}

func hashEvidencePathWithOptions(
	hash interface{ Write([]byte) (int, error) },
	path string,
	includeMTime bool,
) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		_, _ = fmt.Fprintf(hash, "missing %s\n", path)
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.Walk(path, func(candidate string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return hashEvidenceFile(hash, candidate, info, includeMTime)
		})
	}
	return hashEvidenceFile(hash, path, info, includeMTime)
}

type evidenceHasher interface {
	Write([]byte) (int, error)
}

func hashEvidenceFile(
	hash evidenceHasher,
	path string,
	info os.FileInfo,
	includeMTime bool,
) error {
	_, _ = fmt.Fprintf(
		hash,
		"%s %s %d %o\n",
		path,
		info.Mode().Type(),
		info.Size(),
		info.Mode().Perm(),
	)
	if includeMTime {
		_, _ = fmt.Fprintf(hash, "mtime %s\n", info.ModTime().UTC().Format(time.RFC3339Nano))
	}
	if info.IsDir() {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(hash, "link %s\n", link)
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, _ = hash.Write(data)
	_, _ = hash.Write([]byte("\n"))
	return nil
}
