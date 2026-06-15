package target

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/applauselab/bachkator/internal/evidence"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/state"
)

type agentHandler struct{}

type agentPolicyReport struct {
	Mode      string                 `json:"mode"`
	Role      string                 `json:"role,omitempty"`
	Status    string                 `json:"status"`
	Subject   model.AgentSubject     `json:"subject,omitempty"`
	Metrics   []state.QualityMetric  `json:"metrics,omitempty"`
	Findings  []state.QualityFinding `json:"findings,omitempty"`
	Message   string                 `json:"message,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

type agentAttemptReport struct {
	Attempt                 int      `json:"attempt"`
	Commit                  string   `json:"commit,omitempty"`
	FeedbackBundle          string   `json:"feedback_bundle,omitempty"`
	PolicyPassed            bool     `json:"policy_passed"`
	ProviderType            string   `json:"provider_type,omitempty"`
	ProviderSessionID       string   `json:"provider_session_id,omitempty"`
	ProviderEventsPath      string   `json:"provider_events_path,omitempty"`
	ProviderSessionPath     string   `json:"provider_session_path,omitempty"`
	ProviderSummaryPath     string   `json:"provider_summary_path,omitempty"`
	ProviderExecutedCommand []string `json:"provider_executed_command,omitempty"`
	ProviderWorkspaceDirty  *bool    `json:"provider_workspace_dirty,omitempty"`
	ReportPaths             []string `json:"report_paths"`
	LogPaths                []string `json:"log_paths"`
}

type agentFeedbackBundle struct {
	Attempt                int      `json:"attempt"`
	Verdict                string   `json:"verdict"`
	FailedGates            []string `json:"failed_gates"`
	Findings               []string `json:"findings"`
	RequiredTargetFailures []string `json:"required_target_failures"`
	ReviewerSummaries      []string `json:"reviewer_summaries"`
	ReportPaths            []string `json:"report_paths"`
	LogPaths               []string `json:"log_paths"`
	CreatedAt              string   `json:"created_at"`
}

func (agentHandler) Type() model.TargetType { return model.TargetTypeAgent }

func (agentHandler) Runnable(spec model.TargetSpec) bool {
	body, ok := spec.Body.(model.AgentSpec)
	return ok &&
		(body.Mode == "implement" || body.Mode == "merge" || body.Mode == "plan") &&
		providerRunnable(body.Provider)
}

func (agentHandler) Describe(_ context.Context, req DescribeRequest) (RunDescription, error) {
	body, ok := req.Spec.Body.(model.AgentSpec)
	if !ok {
		return RunDescription{}, fmt.Errorf(
			"target %q has %s body, want agent",
			req.Spec.Name,
			req.Spec.TargetType(),
		)
	}
	command := providerBaseCommand(body.Provider)
	command = append(command, "<generated-prompt>")
	operation := strings.Join(command, " ")
	if maxAttempts(body) > 1 {
		operation = fmt.Sprintf("%s (improve max_attempts=%d)", operation, maxAttempts(body))
	}
	return RunDescription{Operation: operation}, nil
}

func (h agentHandler) Execute(ctx context.Context, req ExecuteRequest) error {
	body, ok := req.Spec.Body.(model.AgentSpec)
	if !ok {
		return fmt.Errorf("target %q has %s body, want agent", req.Spec.Name, req.Spec.TargetType())
	}
	if body.Mode == "merge" {
		return h.executeMerge(ctx, req, body)
	}
	if body.Mode != "implement" && body.Mode != "plan" {
		return fmt.Errorf(
			"target %q mode %q is not supported in this phase",
			req.Spec.Name,
			body.Mode,
		)
	}
	runDirectory := req.Env["BACH_RUN_DIRECTORY"]
	if runDirectory == "" {
		return fmt.Errorf("target %q missing BACH_RUN_DIRECTORY", req.Spec.Name)
	}
	workspace, err := evidence.ResolveWorkspace(req.WorkDir, body.Workspace.Path)
	if err != nil {
		return err
	}
	sourceCommit, err := gitpkg.Head(ctx, req.WorkDir)
	if err != nil {
		return fmt.Errorf("project git commit: %w", err)
	}
	if err := h.prepareWorkspace(
		ctx,
		req.WorkDir,
		workspace,
		body.Git.Branch,
		sourceCommit,
	); err != nil {
		return err
	}

	attempts := maxAttempts(body)
	feedbackPath := ""
	history := make([]agentAttemptReport, 0, attempts)
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptDir := filepath.Join(runDirectory, fmt.Sprintf("attempt-%d", attempt))
		if err := os.MkdirAll(attemptDir, 0o755); err != nil {
			return err
		}
		beforeCommit, err := gitpkg.Head(ctx, workspace)
		if err != nil {
			return fmt.Errorf("agent workspace commit: %w", err)
		}
		artifacts, err := h.writeArtifacts(req, body, workspace, attempt, attemptDir, feedbackPath)
		if err != nil {
			return err
		}
		var previous *agentAttemptReport
		if len(history) > 0 {
			previous = &history[len(history)-1]
		}
		afterCommit, providerArtifacts, attemptErr := h.runProviderAttempt(
			ctx,
			req,
			body,
			workspace,
			artifacts,
			attempt,
			attempts,
			attemptDir,
			feedbackPath,
			previous,
		)
		artifacts.Provider = providerArtifacts
		if attemptErr == nil {
			afterCommit, attemptErr = gitpkg.Head(ctx, workspace)
		}
		if attemptErr == nil && body.Git.Commit == "required" && afterCommit == beforeCommit &&
			body.Mode != "plan" {
			attemptErr = fmt.Errorf("target %q requires provider to create a commit", req.Spec.Name)
		}
		if attemptErr == nil {
			attemptErr = h.validateGitEvidence(
				ctx,
				workspace,
				body.Git.Branch,
				beforeCommit,
				afterCommit,
				body.Mode,
			)
		}
		if attemptErr == nil && body.Mode != "plan" {
			attemptErr = validateAgentReport(
				artifacts.ReportPath,
				req.Spec.Name,
				req.WorkDir,
				body,
				workspace,
				afterCommit,
				attempt,
			)
		}
		if attemptErr == nil && body.Mode != "plan" {
			attemptErr = validateAgentReportStatus(artifacts.ReportPath)
		}
		policyTarget := generatedPolicyTargetName(body.Policy.Name, req.Spec.Name)
		stopAfterAttempt := false
		if attemptErr != nil && body.Mode != "plan" && workspaceDirty(ctx, workspace) &&
			body.Provider.Type != "opencode" {
			attemptErr = fmt.Errorf(
				"%w; agent workspace %q has uncommitted changes after failed attempt",
				attemptErr,
				workspace,
			)
			stopAfterAttempt = true
		}
		if attemptErr == nil && body.Policy.Name != "" {
			if req.RunPolicyTarget == nil {
				attemptErr = fmt.Errorf(
					"target %q has policy but runner cannot execute generated policy target",
					req.Spec.Name,
				)
			} else {
				attemptErr = req.RunPolicyTarget(ctx, PolicyTargetRequest{Target: policyTarget})
			}
		}
		report := agentAttemptReport{
			Attempt:                 attempt,
			Commit:                  afterCommit,
			PolicyPassed:            attemptErr == nil,
			ProviderType:            artifacts.Provider.Type,
			ProviderSessionID:       artifacts.Provider.SessionID,
			ProviderEventsPath:      artifacts.Provider.EventsPath,
			ProviderSessionPath:     artifacts.Provider.SessionPath,
			ProviderSummaryPath:     artifacts.Provider.SummaryPath,
			ProviderExecutedCommand: artifacts.Provider.ExecutedCommand,
			ReportPaths:             []string{artifacts.ReportPath},
			LogPaths:                []string{filepath.Join(runDirectory, "agent.log")},
		}
		if artifacts.Provider.Type != "" {
			dirty := artifacts.Provider.WorkspaceDirty
			report.ProviderWorkspaceDirty = &dirty
		}
		if attemptErr != nil && attempts > 1 {
			feedbackPath, err = writeAgentFeedbackBundle(
				attemptDir,
				attempt,
				attemptErr,
				artifacts.ReportPath,
				report.LogPaths[0],
				req.now(),
			)
			if err != nil {
				return err
			}
			report.FeedbackBundle = feedbackPath
		}
		history = append(history, report)
		if err := writeAgentAttemptHistory(runDirectory, history); err != nil {
			return err
		}
		if attemptErr == nil {
			return nil
		}
		lastErr = attemptErr
		if stopAfterAttempt {
			break
		}
	}
	if attempts == 1 {
		return fmt.Errorf("target %q failed: %w", req.Spec.Name, lastErr)
	}
	return fmt.Errorf(
		"target %q failed after %d attempts; latest feedback bundle %q: %w",
		req.Spec.Name,
		attempts,
		feedbackPath,
		lastErr,
	)
}

func (h agentHandler) runProviderAttempt(
	ctx context.Context,
	req ExecuteRequest,
	body model.AgentSpec,
	workspace string,
	artifacts agentArtifacts,
	attempt int,
	attempts int,
	attemptDir string,
	feedbackPath string,
	previous *agentAttemptReport,
) (string, providerAttemptArtifacts, error) {
	result, err := h.runProvider(
		ctx,
		req,
		body,
		workspace,
		artifacts,
		attempt,
		attempts,
		attemptDir,
		feedbackPath,
		previous,
	)
	if err != nil {
		commit, _ := gitpkg.Head(ctx, workspace)
		return commit, result.Artifacts, err
	}
	commit, err := gitpkg.Head(ctx, workspace)
	if err != nil {
		return "", result.Artifacts, fmt.Errorf("agent workspace commit: %w", err)
	}
	return commit, result.Artifacts, nil
}
