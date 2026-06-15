package target

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/evidence"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
)

type agentArtifacts struct {
	PromptPath  string
	ContextPath string
	ReportPath  string
	Provider    providerAttemptArtifacts
}

func (agentHandler) writeArtifacts(
	req ExecuteRequest,
	body model.AgentSpec,
	workspace string,
	attempt int,
	attemptDir string,
	feedbackPath string,
) (agentArtifacts, error) {
	artifacts := agentArtifacts{
		PromptPath:  filepath.Join(attemptDir, "agent-prompt.md"),
		ContextPath: filepath.Join(attemptDir, "agent-context.json"),
		ReportPath:  filepath.Join(attemptDir, "agent-report.json"),
	}
	contextValue := map[string]any{
		"target": req.Spec.Name, "mode": body.Mode, "attempt": attempt,
		"max_attempts": maxAttempts(body), "feedback_bundle": feedbackPath,
		"provider": body.Provider.Name, "provider_type": body.Provider.Type,
		"provider_command": providerBaseCommand(body.Provider), "prompt": body.Prompt.Path,
		"plan": body.Plan, "workspace": workspace, "branch": body.Git.Branch,
		"report_path": artifacts.ReportPath, "context_path": artifacts.ContextPath,
	}
	if err := evidence.WriteJSONArtifact(artifacts.ContextPath, contextValue); err != nil {
		return agentArtifacts{}, err
	}
	prompt, err := renderAgentPrompt(req.WorkDir, body, workspace, artifacts, attempt, feedbackPath)
	if err != nil {
		return agentArtifacts{}, err
	}
	if err := evidence.WritePrivateFile(artifacts.PromptPath, []byte(prompt)); err != nil {
		return agentArtifacts{}, err
	}
	return artifacts, nil
}

func renderAgentPrompt(
	root string,
	body model.AgentSpec,
	workspace string,
	artifacts agentArtifacts,
	attempt int,
	feedbackPath string,
) (string, error) {
	var builder strings.Builder
	builder.WriteString("# Bach Agent Implementation Attempt\n\n")
	builder.WriteString("Attempt: ")
	builder.WriteString(strconv.Itoa(attempt))
	builder.WriteString("\n\n")
	if feedbackPath != "" {
		builder.WriteString("Feedback bundle: ")
		builder.WriteString(feedbackPath)
		builder.WriteString("\n\n")
	}
	if body.Role != "" {
		builder.WriteString("Role: ")
		builder.WriteString(body.Role)
		builder.WriteString("\n\n")
	}
	if err := writeAgentPromptBody(&builder, root, body); err != nil {
		return "", err
	}
	if body.Mode != "plan" {
		planPath, err := evidence.ResolveProjectFile(root, body.Plan)
		if err != nil {
			return "", err
		}
		planContent, err := os.ReadFile(planPath)
		if err != nil {
			return "", fmt.Errorf("read plan %q: %w", body.Plan, err)
		}
		builder.WriteString("## Work Plan\n\n" + "Plan path: ")
		builder.WriteString(planPath)
		builder.WriteString("\n\n")
		builder.Write(planContent)
		builder.WriteString("\n\n")
	}
	builder.WriteString("## Bach Artifacts\n\nWorkspace: ")
	builder.WriteString(workspace)
	builder.WriteString("\nContext path: ")
	builder.WriteString(artifacts.ContextPath)
	builder.WriteString("\nReport path: ")
	builder.WriteString(artifacts.ReportPath)
	builder.WriteString("\nPrompt path: ")
	builder.WriteString(artifacts.PromptPath)
	builder.WriteString("\n\n")
	if body.Git.Commit == "required" {
		builder.WriteString(
			"Commit instructions: create at least one git commit in the workspace " +
				"before writing the Agent Report.\n\n",
		)
	}
	builder.WriteString(implementationReportContract(artifacts.ReportPath, body))
	return builder.String(), nil
}

func writeAgentPromptBody(builder *strings.Builder, root string, body model.AgentSpec) error {
	if body.Prompt.Path == "" {
		builder.WriteString(
			"## Default Prompt\n\nImplement the requested plan in the managed workspace.\n\n",
		)
		return nil
	}
	promptPath, err := evidence.ResolveProjectFile(root, body.Prompt.Path)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(promptPath)
	if err != nil {
		return fmt.Errorf("read prompt %q: %w", body.Prompt.Path, err)
	}
	builder.WriteString("## Reusable Prompt\n\n")
	builder.Write(content)
	builder.WriteString("\n\n")
	return nil
}

func implementationReportContract(reportPath string, body model.AgentSpec) string {
	providerType := body.Provider.Type
	if providerType == "" {
		providerType = "agent"
	}
	providerCommand, err := json.Marshal(providerBaseCommand(body.Provider))
	if err != nil {
		providerCommand = []byte(`[]`)
	}
	if body.Mode == "plan" {
		return fmt.Sprintf(`## Required Planning Output

You are in planning mode. Write the plan Markdown file to $BACH_PLAN_OUTPUT_PATH (%s).
The plan file must be valid Bachkator Plan Markdown with optional YAML frontmatter.
No Agent Target completion report is required for planning mode.
`, body.Plan)
	}
	return fmt.Sprintf(`## Required Implementation Report

Write an Agent Target completion report JSON file to %s with this exact top-level shape:

`+"```json"+`
{
  "target": "$BACH_AGENT_TARGET",
  "provider_name": "<provider-name>",
  "provider_type": %q,
  "provider_command": %s,
  "mode": "implement",
  "status": "passed",
  "attempt": 1,
  "workspace": "$BACH_AGENT_WORKSPACE",
  "branch": "<current-branch>",
  "commit": "<new-commit-sha>",
  "changed_files": ["path/to/file"],
  "summary": "Implemented the requested plan."
}
`+"```"+`

Use status "passed" only after the implementation commit is created and the workspace is
clean.
`, reportPath, providerType, providerCommand)
}

func validateAgentReport(
	path string,
	targetName string,
	root string,
	body model.AgentSpec,
	workspace string,
	commit string,
	attempt int,
) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("agent report %q is missing: %w", path, err)
	}
	var report struct {
		Target          string   `json:"target"`
		ProviderName    string   `json:"provider_name"`
		ProviderType    string   `json:"provider_type"`
		ProviderCommand []string `json:"provider_command"`
		Mode            string   `json:"mode"`
		Status          string   `json:"status"`
		Attempt         int      `json:"attempt"`
		Workspace       string   `json:"workspace"`
		Branch          string   `json:"branch"`
		Commit          string   `json:"commit"`
		ChangedFiles    []string `json:"changed_files"`
		Summary         string   `json:"summary"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("agent report %q is invalid JSON: %w", path, err)
	}
	if err := validateAgentReportMetadata(
		report.Target,
		targetName,
		report.ProviderName,
		report.ProviderType,
		body,
	); err != nil {
		return err
	}
	targetCommand := providerBaseCommand(body.Provider)
	expandedCommand := runenv.ExpandSlice(
		targetCommand,
		map[string]string{"BACH_PROJECT_ROOT": root},
	)
	if !providerCommandMatches(report.ProviderCommand, targetCommand) &&
		!providerCommandMatches(report.ProviderCommand, expandedCommand) {
		return fmt.Errorf("agent report provider command evidence does not match target provider")
	}
	if report.Mode != body.Mode {
		return fmt.Errorf("agent report mode = %q, want %q", report.Mode, body.Mode)
	}
	if !validAgentStatus(report.Status) {
		return fmt.Errorf("agent report status %q is invalid", report.Status)
	}
	if report.Attempt != attempt {
		return fmt.Errorf("agent report attempt = %d, want %d", report.Attempt, attempt)
	}
	if report.Workspace != workspace {
		return fmt.Errorf("agent report workspace = %q, want %q", report.Workspace, workspace)
	}
	if report.Branch != body.Git.Branch {
		return fmt.Errorf("agent report branch = %q, want %q", report.Branch, body.Git.Branch)
	}
	if !agentReportCommitMatches(workspace, report.Commit, commit) {
		return fmt.Errorf("agent report commit = %q, want %q", report.Commit, commit)
	}
	if report.Summary == "" {
		return fmt.Errorf("agent report summary is required")
	}
	return nil
}

func validateAgentReportMetadata(
	reportTarget string,
	targetName string,
	providerName string,
	providerType string,
	body model.AgentSpec,
) error {
	if reportTarget != targetName {
		return fmt.Errorf("agent report target = %q, want %q", reportTarget, targetName)
	}
	if providerName != body.Provider.Name || providerType != body.Provider.Type {
		return fmt.Errorf("agent report provider evidence does not match target provider")
	}
	return nil
}

func providerCommandMatches(reportCommand []string, targetCommand []string) bool {
	if len(reportCommand) < len(targetCommand) {
		return false
	}
	for i, part := range targetCommand {
		if reportCommand[i] != part {
			return false
		}
	}
	return true
}

func agentReportCommitMatches(workspace string, reportCommit string, expectedCommit string) bool {
	if reportCommit == expectedCommit {
		return true
	}
	if reportCommit == "" || expectedCommit == "" {
		return false
	}
	resolved, err := gitpkg.ResolveCommit(context.Background(), workspace, reportCommit)
	return err == nil && resolved == expectedCommit
}

func validAgentStatus(status string) bool {
	switch status {
	case "passed", "failed", "blocked", "partial":
		return true
	default:
		return false
	}
}

func validateAgentReportStatus(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var report struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	if report.Status != "passed" {
		return fmt.Errorf("agent report status %q does not pass the target", report.Status)
	}
	return nil
}

func writeAgentFeedbackBundle(
	attemptDir string,
	attempt int,
	attemptErr error,
	reportPath string,
	logPath string,
	createdAt time.Time,
) (string, error) {
	findings := nonEmptyLines([]byte(attemptErr.Error()))
	if len(findings) == 0 {
		findings = []string{"agent attempt failed without output"}
	}
	bundle := agentFeedbackBundle{
		Attempt: attempt, Verdict: "failed", FailedGates: findings, Findings: findings,
		RequiredTargetFailures: findings, ReviewerSummaries: findings,
		ReportPaths: []string{reportPath}, LogPaths: []string{logPath},
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}
	path := filepath.Join(attemptDir, "feedback-bundle.json")
	return path, evidence.WriteJSONArtifact(path, bundle)
}

func writeAgentAttemptHistory(runDirectory string, reports []agentAttemptReport) error {
	return evidence.WriteJSONArtifact(filepath.Join(runDirectory, "attempt-history.json"), reports)
}

func nonEmptyLines(data []byte) []string {
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
