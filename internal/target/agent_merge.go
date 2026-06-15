package target

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/applauselab/bachkator/internal/evidence"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
)

type mergePolicyEvidence struct {
	Schema           string `json:"schema"`
	RunID            string `json:"run_id"`
	Target           string `json:"target"`
	PolicyTarget     string `json:"policy_target"`
	SubjectWorkspace string `json:"subject_workspace"`
	SubjectCommit    string `json:"subject_commit"`
	Verdict          string `json:"verdict"`
	Path             string `json:"path"`
}

func (h agentHandler) executeMerge(
	ctx context.Context,
	req ExecuteRequest,
	body model.AgentSpec,
) error {
	runDirectory := req.Env["BACH_RUN_DIRECTORY"]
	if runDirectory == "" {
		return fmt.Errorf("target %q missing BACH_RUN_DIRECTORY", req.Spec.Name)
	}
	subject := body.SubjectInfo
	subjectWorkspace, err := evidence.ResolveWorkspace(req.WorkDir, subject.Workspace)
	if err != nil {
		return err
	}
	subjectCommit, err := gitpkg.Head(ctx, subjectWorkspace)
	if err != nil {
		return fmt.Errorf("subject workspace commit: %w", err)
	}
	if err := validateMergeSubjectWorkspace(
		ctx,
		subjectWorkspace,
		subject.Branch,
		subjectCommit,
	); err != nil {
		return err
	}
	policy, err := latestPassingPolicyEvidence(
		req.WorkDir,
		req.StatePath,
		body.Subject,
		subject.PolicyTarget,
		subjectWorkspace,
		subjectCommit,
	)
	if err != nil {
		return err
	}
	artifacts, err := h.writeMergeArtifacts(
		req,
		body,
		subject,
		subjectWorkspace,
		subjectCommit,
		policy,
	)
	if err != nil {
		return err
	}
	beforeMergeStatus, err := gitStatusExcludingBach(ctx, req.WorkDir)
	if err != nil {
		return fmt.Errorf("merge checkout status before provider: %w", err)
	}
	if err := h.runMergeProvider(
		ctx,
		req,
		body,
		subject,
		subjectWorkspace,
		subjectCommit,
		artifacts,
		policy,
	); err != nil {
		return err
	}
	if status, err := gitStatusExcludingBach(ctx, req.WorkDir); err != nil {
		return fmt.Errorf("merge checkout status: %w", err)
	} else if status != beforeMergeStatus {
		return fmt.Errorf("merge provider changed main checkout status: %s", status)
	}
	return validateMergeReport(
		artifacts.ReportPath,
		req.Spec.Name,
		req.WorkDir,
		body,
		subjectWorkspace,
		subjectCommit,
	)
}

func latestPassingPolicyEvidence(
	root string,
	statePath string,
	subject string,
	expectedPolicyTarget string,
	subjectWorkspace string,
	subjectCommit string,
) (mergePolicyEvidence, error) {
	name := strings.NewReplacer("/", "-", ":", "-", " ", "-").Replace(subject) + ".json"
	base := filepath.Join(root, ".bach", "artifacts", "policies")
	var latest mergePolicyEvidence
	var latestInfo os.FileInfo
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || filepath.Base(path) != name {
			return nil
		}
		linkInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if linkInfo.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var evidence mergePolicyEvidence
		if err := json.Unmarshal(data, &evidence); err != nil {
			return err
		}
		if filepath.Base(filepath.Dir(path)) != evidence.RunID {
			return nil
		}
		if evidence.Schema == "bach.applied_policy.v1" && evidence.Target == subject &&
			evidence.PolicyTarget == expectedPolicyTarget &&
			samePath(evidence.SubjectWorkspace, subjectWorkspace) &&
			evidence.SubjectCommit == subjectCommit &&
			(latestInfo == nil || info.ModTime().After(latestInfo.ModTime())) {
			evidence.Path = path
			latest = evidence
			latestInfo = info
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return mergePolicyEvidence{}, err
	}
	if latest.Path == "" {
		return mergePolicyEvidence{}, fmt.Errorf(
			"merge target requires passing applied policy verdict for subject %q",
			subject,
		)
	}
	if latest.Verdict != "passed" || !policyEvidenceRunPassed(root, statePath, latest) {
		return mergePolicyEvidence{}, fmt.Errorf(
			"merge target requires latest applied policy verdict to pass for subject %q",
			subject,
		)
	}
	return latest, nil
}

func policyEvidenceRunPassed(root string, statePath string, evidence mergePolicyEvidence) bool {
	if evidence.RunID == "" || evidence.PolicyTarget == "" {
		return false
	}
	store, err := openTrustedEvidenceStore(root, statePath)
	if err != nil {
		return false
	}
	defer func() { _ = store.Close() }()
	snapshot, err := store.Load()
	if err != nil {
		return false
	}
	for _, run := range snapshot.Runs {
		if run.ID != evidence.RunID || run.Status != "success" {
			continue
		}
		targetRun, ok := run.Targets[evidence.PolicyTarget]
		return ok && targetRun.Status == "success"
	}
	return false
}

func (agentHandler) writeMergeArtifacts(
	req ExecuteRequest,
	body model.AgentSpec,
	subject model.AgentSubject,
	subjectWorkspace string,
	subjectCommit string,
	policy mergePolicyEvidence,
) (agentArtifacts, error) {
	artifacts := agentArtifacts{
		PromptPath:  filepath.Join(req.Env["BACH_RUN_DIRECTORY"], "merge-prompt.md"),
		ContextPath: filepath.Join(req.Env["BACH_RUN_DIRECTORY"], "merge-context.json"),
		ReportPath:  filepath.Join(req.Env["BACH_RUN_DIRECTORY"], "merge-report.json"),
	}
	contextValue := map[string]any{
		"target": req.Spec.Name, "mode": body.Mode, "provider": body.Provider.Name,
		"provider_type": body.Provider.Type, "provider_command": providerBaseCommand(body.Provider),
		"prompt": body.Prompt.Path, "workspace": req.WorkDir, "report_path": artifacts.ReportPath,
		"context_path": artifacts.ContextPath,
		"subject": map[string]any{
			"target": body.Subject, "branch": subject.Branch,
			"commit": subjectCommit, "workspace": subjectWorkspace, "plan": subject.Plan,
		},
		"policy": policy,
	}
	if err := evidence.WriteJSONArtifact(artifacts.ContextPath, contextValue); err != nil {
		return agentArtifacts{}, err
	}
	prompt, err := renderMergePrompt(
		req.WorkDir,
		body,
		subject,
		subjectWorkspace,
		subjectCommit,
		artifacts,
		policy,
	)
	if err != nil {
		return agentArtifacts{}, err
	}
	if err := evidence.WritePrivateFile(artifacts.PromptPath, []byte(prompt)); err != nil {
		return agentArtifacts{}, err
	}
	return artifacts, nil
}

func renderMergePrompt(
	root string,
	body model.AgentSpec,
	subject model.AgentSubject,
	subjectWorkspace string,
	subjectCommit string,
	artifacts agentArtifacts,
	policy mergePolicyEvidence,
) (string, error) {
	var builder strings.Builder
	builder.WriteString("# Bach Merge Agent\n\n")
	if body.Prompt.Path != "" {
		path, err := evidence.ResolveProjectFile(root, body.Prompt.Path)
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read prompt %q: %w", body.Prompt.Path, err)
		}
		builder.WriteString("## User Prompt\n\n")
		builder.Write(content)
		builder.WriteString("\n\n")
	} else {
		builder.WriteString(
			"## Default Prompt\n\nMerge the reviewed subject branch or open a merge PR.\n\n",
		)
	}
	builder.WriteString("Subject target: ")
	builder.WriteString(body.Subject)
	builder.WriteString("\nSubject branch: ")
	builder.WriteString(subject.Branch)
	builder.WriteString("\nSubject commit: ")
	builder.WriteString(subjectCommit)
	builder.WriteString("\nSubject workspace: ")
	builder.WriteString(subjectWorkspace)
	builder.WriteString("\nSubject plan: ")
	builder.WriteString(subject.Plan)
	builder.WriteString("\nPolicy verdict: ")
	builder.WriteString(policy.Verdict)
	builder.WriteString("\nPolicy evidence: ")
	builder.WriteString(policy.Path)
	builder.WriteString("\nContext path: ")
	builder.WriteString(artifacts.ContextPath)
	builder.WriteString("\nReport path: ")
	builder.WriteString(artifacts.ReportPath)
	builder.WriteString("\n\n")
	builder.WriteString(mergeReportContract(artifacts.ReportPath))
	return builder.String(), nil
}

func mergeReportContract(reportPath string) string {
	return fmt.Sprintf(`## Required Merge Report

Write an Agent Target completion report JSON file to %s. Merge outcome evidence must be
top-level JSON, not only prose, message text, or findings.

`+"```json"+`
{
  "target": "$BACH_AGENT_TARGET",
  "provider_name": "<provider-name>",
  "provider_type": "agent",
  "provider_command": ["<argv0>", "<argv1>"],
  "mode": "merge",
  "status": "passed",
  "subject": {
    "target": "$BACH_AGENT_SUBJECT_TARGET",
    "workspace": "$BACH_AGENT_SUBJECT_WORKSPACE",
    "commit": "$BACH_AGENT_SUBJECT_COMMIT"
  },
  "pr_url": "https://example.test/pull/123",
  "target_branch_commit": "<optional-target-branch-commit>",
  "merge_commit": "<optional-merge-commit>",
  "summary": "Opened PR or merged the reviewed subject branch."
}
`+"```"+`

Rules:

- Use status "passed" only after the merge or PR operation and required verification
  succeeded.
- Include at least one of "pr_url", "target_branch_commit", or "merge_commit".
- "pr_url" must be an absolute URL.
- Commit evidence must be real commit SHAs reachable in the main checkout and must have
  the reviewed subject commit as an ancestor.
`, reportPath)
}

func (agentHandler) runMergeProvider(
	ctx context.Context,
	req ExecuteRequest,
	body model.AgentSpec,
	subject model.AgentSubject,
	subjectWorkspace string,
	subjectCommit string,
	artifacts agentArtifacts,
	policy mergePolicyEvidence,
) error {
	providerEnv := copyEnv(req.Env)
	providerEnv["BACH_AGENT_CONTEXT_PATH"] = artifacts.ContextPath
	providerEnv["BACH_AGENT_REPORT_PATH"] = artifacts.ReportPath
	providerEnv["BACH_AGENT_WORKSPACE"] = req.WorkDir
	providerEnv["BACH_AGENT_PROMPT_PATH"] = artifacts.PromptPath
	providerEnv["BACH_AGENT_TARGET"] = req.Spec.Name
	providerEnv["BACH_AGENT_MODE"] = body.Mode
	providerEnv["BACH_AGENT_ROLE"] = body.Role
	providerEnv["BACH_AGENT_SUBJECT_TARGET"] = body.Subject
	providerEnv["BACH_AGENT_SUBJECT_BRANCH"] = subject.Branch
	providerEnv["BACH_AGENT_SUBJECT_COMMIT"] = subjectCommit
	providerEnv["BACH_AGENT_SUBJECT_WORKSPACE"] = subjectWorkspace
	providerEnv["BACH_AGENT_POLICY_EVIDENCE"] = policy.Path
	providerEnv["BACH_PROJECT_ROOT"] = req.WorkDir
	beforeEvidence, err := trustedEvidenceSnapshot(req.WorkDir, req.StatePath)
	if err != nil {
		return err
	}
	command := runenv.ExpandSlice(providerBaseCommand(body.Provider), providerEnv)
	command = append(command, artifacts.PromptPath)
	if len(command) == 0 {
		return fmt.Errorf("target %q provider command is empty", req.Spec.Name)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = req.WorkDir
	cmd.Env = runenv.List(providerEnv)
	cmd.Stdout = req.Stdout
	cmd.Stderr = req.Stderr
	runErr := cmd.Run()
	evidenceErr := validateTrustedEvidenceUnchanged(req.WorkDir, req.StatePath, beforeEvidence)
	if evidenceErr != nil {
		if runErr != nil {
			return fmt.Errorf("%w; %w", runErr, evidenceErr)
		}
		return evidenceErr
	}
	return runErr
}

func validateMergeReport(
	path string,
	targetName string,
	root string,
	body model.AgentSpec,
	subjectWorkspace string,
	subjectCommit string,
) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("merge agent report %q is missing: %w", path, err)
	}
	var report struct {
		Target             string             `json:"target"`
		ProviderName       string             `json:"provider_name"`
		ProviderType       string             `json:"provider_type"`
		ProviderCommand    []string           `json:"provider_command"`
		Mode               string             `json:"mode"`
		Status             string             `json:"status"`
		Subject            model.AgentSubject `json:"subject"`
		PRURL              string             `json:"pr_url"`
		TargetBranchCommit string             `json:"target_branch_commit"`
		MergeCommit        string             `json:"merge_commit"`
		Summary            string             `json:"summary"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("merge agent report %q is invalid JSON: %w", path, err)
	}
	if report.Target != targetName {
		return fmt.Errorf("merge agent report target = %q, want %q", report.Target, targetName)
	}
	expandedCommand := runenv.ExpandSlice(
		providerBaseCommand(body.Provider),
		map[string]string{"BACH_PROJECT_ROOT": root},
	)
	reportCommand := strings.Join(report.ProviderCommand, "\x00")
	bodyCommand := strings.Join(providerBaseCommand(body.Provider), "\x00")
	resolvedCommand := strings.Join(expandedCommand, "\x00")
	if report.ProviderName != body.Provider.Name || report.ProviderType != body.Provider.Type ||
		(reportCommand != bodyCommand && reportCommand != resolvedCommand) {
		return fmt.Errorf("merge agent report provider evidence does not match target provider")
	}
	if report.Mode != "merge" {
		return fmt.Errorf("merge agent report mode = %q, want merge", report.Mode)
	}
	if report.Status != "passed" && report.Status != "pass" {
		return fmt.Errorf("merge agent report status %q does not pass the target", report.Status)
	}
	if report.Subject.Target != body.Subject ||
		report.Subject.Workspace != subjectWorkspace ||
		report.Subject.Commit != subjectCommit {
		return fmt.Errorf("merge agent report subject metadata does not match merge subject")
	}
	if report.PRURL == "" && report.TargetBranchCommit == "" && report.MergeCommit == "" {
		return fmt.Errorf(
			"merge agent report must include pr_url, " +
				"target_branch_commit, or merge_commit evidence",
		)
	}
	if report.PRURL != "" && report.TargetBranchCommit == "" && report.MergeCommit == "" {
		return fmt.Errorf(
			"merge agent report pr_url evidence must include " +
				"target_branch_commit or merge_commit",
		)
	}
	if report.PRURL != "" {
		parsed, err := url.Parse(report.PRURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("merge agent report pr_url is not a valid absolute URL")
		}
	}
	if err := validateCommitEvidence(
		root,
		"target_branch_commit",
		report.TargetBranchCommit,
		subjectCommit,
	); err != nil {
		return err
	}
	if err := validateCommitEvidence(
		root,
		"merge_commit",
		report.MergeCommit,
		subjectCommit,
	); err != nil {
		return err
	}
	if report.Summary == "" {
		return fmt.Errorf("merge agent report summary is required")
	}
	return nil
}

func validateCommitEvidence(root string, field string, commit string, subjectCommit string) error {
	if commit == "" {
		return nil
	}
	resolvedCommit, err := gitpkg.ResolveCommit(context.Background(), root, commit)
	if err != nil {
		return fmt.Errorf("merge agent report %s %q is not a reachable commit", field, commit)
	}
	if subjectCommit != "" {
		err := gitpkg.IsAncestor(context.Background(), root, subjectCommit, resolvedCommit)
		if err != nil {
			return fmt.Errorf(
				"merge agent report %s %q does not contain subject commit %s",
				field,
				commit,
				subjectCommit,
			)
		}
	}
	return nil
}
