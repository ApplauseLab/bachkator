package target

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/applauselab/bachkator/internal/evidence"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
	"github.com/applauselab/bachkator/internal/state"
)

func runAttachedPolicyRequiredTargets(
	ctx context.Context,
	req ExecuteRequest,
	agent model.AgentSpec,
	subject model.AgentSubject,
	runDirectory string,
	attemptDir string,
) error {
	if req.RunRequiredTargets == nil {
		return fmt.Errorf("target %q cannot run attached policy required targets", req.Spec.Name)
	}
	err := req.RunRequiredTargets(ctx, RequiredTargetsRequest{
		Targets:       agent.Policy.RequiredTargets,
		WorkDir:       subject.Workspace,
		Subject:       subject.Target,
		SubjectCommit: subject.Commit,
		PolicyNode:    agent.Policy.Name,
	})
	if err != nil {
		finding := state.QualityFinding{
			Kind:     "policy-required-target",
			Severity: "error",
			Rule:     "required-target-failed",
			Message:  err.Error(),
		}
		return writeRequiredTargetFailure(
			runDirectory,
			attemptDir,
			subject,
			finding,
			err,
			req.now(),
		)
	}
	if err := validateReviewedWorkspaceUnchanged(ctx, subject.Workspace, subject); err != nil {
		finding := state.QualityFinding{
			Kind:     "policy-required-target",
			Severity: "error",
			Rule:     "required-target-mutated-workspace",
			Message:  err.Error(),
		}
		return writeRequiredTargetFailure(
			runDirectory,
			attemptDir,
			subject,
			finding,
			err,
			req.now(),
		)
	}
	return nil
}

func writeRequiredTargetFailure(
	runDirectory string,
	attemptDir string,
	subject model.AgentSubject,
	finding state.QualityFinding,
	cause error,
	createdAt time.Time,
) error {
	if writeErr := writePolicyReportWithFindings(
		runDirectory,
		attemptDir,
		subject,
		[]state.QualityFinding{finding},
		createdAt,
	); writeErr != nil {
		return writeErr
	}
	return cause
}

func writePassingPolicyReport(
	runDirectory string,
	attemptDir string,
	subject model.AgentSubject,
	gates []model.QualityGateSpec,
	createdAt time.Time,
) error {
	if err := writePolicyReportWithFindings(
		runDirectory,
		attemptDir,
		subject,
		nil,
		createdAt,
	); err != nil {
		return err
	}
	metrics := []state.QualityMetric{
		{Name: "findings.open.count", Value: 0, Unit: "count"},
		{Name: "findings.error.open.count", Value: 0, Unit: "count"},
	}
	return evaluateAgentPolicyGates(subject.Target, gates, metrics)
}

func writePolicyReportWithFindings(
	runDirectory string,
	attemptDir string,
	subject model.AgentSubject,
	findings []state.QualityFinding,
	createdAt time.Time,
) error {
	metrics, status := agentPolicyMetrics(findings)
	policyReport := agentPolicyReport{
		Mode:      "review",
		Role:      "policy-aggregator",
		Status:    status,
		Subject:   subject,
		Metrics:   metrics,
		Findings:  findings,
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}
	if err := writeAgentPolicyReport(
		filepath.Join(attemptDir, "policy-report.json"),
		policyReport,
	); err != nil {
		return err
	}
	return writeAgentPolicyReport(filepath.Join(runDirectory, "policy-report.json"), policyReport)
}

func runReviewPolicy(
	ctx context.Context,
	agent model.AgentSpec,
	subject model.AgentSubject,
	req ExecuteRequest,
	workspace string,
	runDirectory string,
	attemptDir string,
) error {
	beforeProject, err := projectCheckoutSnapshot(ctx, req.WorkDir)
	if err != nil {
		return fmt.Errorf("project checkout before reviewers: %w", err)
	}
	findings := runReviewerProviders(ctx, agent, subject, req, workspace, attemptDir)
	if err := validateReviewedWorkspaceUnchanged(ctx, workspace, subject); err != nil {
		findings = append(
			findings,
			reviewerFailureFinding(model.AgentSpec{Role: "policy-aggregator"}, err.Error()),
		)
	}
	if err := validateProjectCheckoutSnapshot(ctx, req.WorkDir, beforeProject); err != nil {
		findings = append(
			findings,
			reviewerFailureFinding(model.AgentSpec{Role: "policy-aggregator"}, err.Error()),
		)
	}
	metrics, status := agentPolicyMetrics(findings)
	createdAt := req.now()
	policyReport := agentPolicyReport{
		Mode:      "review",
		Role:      "policy-aggregator",
		Status:    status,
		Subject:   subject,
		Metrics:   metrics,
		Findings:  findings,
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}
	attemptReportPath := filepath.Join(attemptDir, "policy-report.json")
	if err := writeAgentPolicyReport(attemptReportPath, policyReport); err != nil {
		return err
	}
	runReportPath := filepath.Join(runDirectory, "policy-report.json")
	if err := writeAgentPolicyReport(runReportPath, policyReport); err != nil {
		return err
	}
	if status == "failed" {
		return fmt.Errorf(
			"agent policy failed with %d blocking findings: %s",
			errorFindingCount(findings),
			findingMessages(findings),
		)
	}
	return evaluateAgentPolicyGates(subject.Target, agent.Policy.Gates, metrics)
}

func runReviewerProviders(
	ctx context.Context,
	agent model.AgentSpec,
	subject model.AgentSubject,
	req ExecuteRequest,
	workspace string,
	attemptDir string,
) []state.QualityFinding {
	var mu sync.Mutex
	findings := []state.QualityFinding{}
	var wg sync.WaitGroup
	for index, reviewer := range agent.Policy.ReviewerSpecs {
		wg.Add(1)
		go func(index int, reviewer model.AgentSpec) {
			defer wg.Done()
			reportName := fmt.Sprintf("review-%02d-%s.json", index+1, safeName(reviewer.Role))
			reviewerFindings := runOneReviewerProvider(
				ctx,
				reviewer,
				subject,
				req,
				workspace,
				filepath.Join(attemptDir, reportName),
			)
			mu.Lock()
			findings = append(findings, reviewerFindings...)
			mu.Unlock()
		}(index, reviewer)
	}
	wg.Wait()
	return findings
}

func runOneReviewerProvider(
	ctx context.Context,
	reviewer model.AgentSpec,
	subject model.AgentSubject,
	req ExecuteRequest,
	workspace string,
	reportPath string,
) []state.QualityFinding {
	if err := runReviewerProvider(ctx, reviewer, subject, req, workspace, reportPath); err != nil {
		return []state.QualityFinding{reviewerFailureFinding(reviewer, err.Error())}
	}
	report, err := readAgentPolicyReport(reportPath)
	if err != nil {
		return []state.QualityFinding{reviewerFailureFinding(reviewer, err.Error())}
	}
	if report.Mode != "review" {
		return []state.QualityFinding{reviewerFailureFinding(
			reviewer,
			fmt.Sprintf("reviewer report mode %q, want review", report.Mode),
		)}
	}
	if !sameSubject(report.Subject, subject) {
		return []state.QualityFinding{reviewerFailureFinding(
			reviewer,
			"reviewer report subject metadata does not match policy subject",
		)}
	}
	findings := append([]state.QualityFinding(nil), report.Findings...)
	if report.Status != "" && report.Status != "pass" && report.Status != "passed" {
		findings = append(findings, reviewerFailureFinding(
			reviewer,
			"reviewer reported non-pass status: "+report.Status,
		))
	}
	return findings
}

func agentPolicyMetrics(findings []state.QualityFinding) ([]state.QualityMetric, string) {
	errorCount := errorFindingCount(findings)
	status := "passed"
	if errorCount > 0 {
		status = "failed"
	}
	return []state.QualityMetric{
		{Name: "findings.open.count", Value: float64(len(findings)), Unit: "count"},
		{Name: "findings.error.open.count", Value: float64(errorCount), Unit: "count"},
	}, status
}

func errorFindingCount(findings []state.QualityFinding) int {
	errorCount := 0
	for _, finding := range findings {
		if finding.Severity == "error" || finding.Severity == "blocking" {
			errorCount++
		}
	}
	return errorCount
}

func evaluateAgentPolicyGates(
	targetName string,
	gates []model.QualityGateSpec,
	metrics []state.QualityMetric,
) error {
	if len(gates) == 0 {
		return nil
	}
	values := map[string]float64{}
	for _, metric := range metrics {
		values[metric.Name] = metric.Value
	}
	for _, gate := range gates {
		value := values[gate.Metric]
		if gate.Min != nil && value < *gate.Min {
			return fmt.Errorf(
				"target %q policy gate %q failed: %.2f < min %.2f",
				targetName,
				gate.Metric,
				value,
				*gate.Min,
			)
		}
		if gate.Max != nil && value > *gate.Max {
			return fmt.Errorf(
				"target %q policy gate %q failed: %.2f > max %.2f",
				targetName,
				gate.Metric,
				value,
				*gate.Max,
			)
		}
	}
	return nil
}

func findingMessages(findings []state.QualityFinding) string {
	messages := make([]string, 0, len(findings))
	for _, finding := range findings {
		if finding.Message != "" {
			messages = append(messages, finding.Message)
		}
	}
	if len(messages) == 0 {
		return "no finding messages"
	}
	return strings.Join(messages, "; ")
}

func runReviewerProvider(
	ctx context.Context,
	reviewer model.AgentSpec,
	subject model.AgentSubject,
	req ExecuteRequest,
	workspace string,
	reportPath string,
) error {
	if !providerRunnable(reviewer.Provider) {
		return fmt.Errorf("reviewer %q provider command is empty", reviewer.Role)
	}
	beforeEvidence, err := trustedEvidenceSnapshot(req.WorkDir, req.StatePath)
	if err != nil {
		return err
	}
	promptPath := reportPath + ".prompt.md"
	prompt, err := renderReviewerPrompt(req.WorkDir, reviewer, subject, reportPath)
	if err != nil {
		return err
	}
	if err := evidence.WritePrivateFile(promptPath, []byte(prompt)); err != nil {
		return err
	}
	providerEnv := reviewerProviderEnv(req, reviewer, subject, workspace, reportPath, promptPath)
	command := runenv.ExpandSlice(providerBaseCommand(reviewer.Provider), providerEnv)
	command = append(command, promptPath)
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = workspace
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

func reviewerProviderEnv(
	req ExecuteRequest,
	reviewer model.AgentSpec,
	subject model.AgentSubject,
	workspace string,
	reportPath string,
	promptPath string,
) map[string]string {
	providerEnv := copyEnv(req.Env)
	providerEnv["BACH_AGENT_MODE"] = reviewer.Mode
	providerEnv["BACH_AGENT_ROLE"] = reviewer.Role
	providerEnv["BACH_AGENT_REPORT_PATH"] = reportPath
	providerEnv["BACH_AGENT_PROMPT_PATH"] = promptPath
	providerEnv["BACH_AGENT_TARGET"] = subject.Target
	providerEnv["BACH_AGENT_WORKSPACE"] = workspace
	providerEnv["BACH_AGENT_SUBJECT_TARGET"] = subject.Target
	providerEnv["BACH_AGENT_SUBJECT_WORKSPACE"] = subject.Workspace
	providerEnv["BACH_AGENT_SUBJECT_COMMIT"] = subject.Commit
	providerEnv["BACH_AGENT_SUBJECT_PLAN"] = subject.Plan
	providerEnv["BACH_PROJECT_ROOT"] = req.WorkDir
	return providerEnv
}

func renderReviewerPrompt(
	root string,
	reviewer model.AgentSpec,
	subject model.AgentSubject,
	reportPath string,
) (string, error) {
	var builder strings.Builder
	builder.WriteString("# Bach Reviewer Agent\n\n")
	if reviewer.Prompt.Path != "" {
		path, err := evidence.ResolveProjectFile(root, reviewer.Prompt.Path)
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read prompt %q: %w", reviewer.Prompt.Path, err)
		}
		builder.WriteString("## User Prompt\n\n")
		builder.Write(content)
		builder.WriteString("\n\n")
	} else {
		builder.WriteString(
			"## Default Prompt\n\nReview the subject changes and emit policy findings.\n\n",
		)
	}
	builder.WriteString("## Subject\n\nTarget: ")
	builder.WriteString(subject.Target)
	builder.WriteString("\nWorkspace: ")
	builder.WriteString(subject.Workspace)
	builder.WriteString("\nCommit: ")
	builder.WriteString(subject.Commit)
	builder.WriteString("\nPlan: ")
	builder.WriteString(subject.Plan)
	builder.WriteString("\n\n")
	builder.WriteString(reviewerReportContract(reviewer.Role, reportPath))
	return builder.String(), nil
}

func reviewerReportContract(role string, reportPath string) string {
	if role == "" {
		role = "reviewer"
	}
	return fmt.Sprintf(`## Required Reviewer Report

Write reviewer quality evidence JSON to %s with this exact top-level shape:

`+"```json"+`
{
  "mode": "review",
  "role": %q,
  "status": "passed",
  "subject": {
    "target": "$BACH_AGENT_SUBJECT_TARGET",
    "workspace": "$BACH_AGENT_SUBJECT_WORKSPACE",
    "commit": "$BACH_AGENT_SUBJECT_COMMIT"
  },
  "metrics": [],
  "findings": [],
  "message": "Review completed."
}
`+"```"+`

Use status "passed" only when there are no blocking findings. Findings use JSON keys
"kind", "severity", "rule", "message", "file", and "line".
`, reportPath, role)
}

func sameSubject(got, want model.AgentSubject) bool {
	return got.Target == want.Target && got.Workspace == want.Workspace && got.Commit == want.Commit
}

func reviewerFailureFinding(reviewer model.AgentSpec, message string) state.QualityFinding {
	return state.QualityFinding{
		Kind:     "agent-review",
		Severity: "error",
		Rule:     "reviewer-evidence-invalid",
		Message:  reviewer.Role + ": " + message,
	}
}

func writeAgentPolicyReport(path string, report agentPolicyReport) error {
	return evidence.WriteJSONArtifact(path, report)
}

func readAgentPolicyReport(path string) (agentPolicyReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentPolicyReport{}, err
	}
	var report agentPolicyReport
	if err := json.Unmarshal(data, &report); err != nil {
		return agentPolicyReport{}, err
	}
	return report, nil
}
