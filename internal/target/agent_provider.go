package target

import (
	"context"
	"fmt"
	"strconv"

	"github.com/applauselab/bachkator/internal/agentprovider"
	"github.com/applauselab/bachkator/internal/model"
)

type providerAttemptArtifacts = agentprovider.Artifacts

type providerRunResult struct {
	Artifacts providerAttemptArtifacts
}

func providerRunnable(provider model.Provider) bool {
	return agentprovider.Runnable(provider)
}

func providerBaseCommand(provider model.Provider) []string {
	return agentprovider.BaseCommand(provider)
}

func (h agentHandler) runProvider(
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
) (providerRunResult, error) {
	providerEnv := agentProviderEnv(
		req,
		body,
		workspace,
		artifacts,
		attempt,
		attempts,
		attemptDir,
		feedbackPath,
	)
	beforeProject, err := projectCheckoutSnapshot(ctx, req.WorkDir)
	if err != nil {
		return providerRunResult{}, fmt.Errorf("project checkout before provider: %w", err)
	}
	beforeEvidence, err := trustedEvidenceSnapshot(req.WorkDir, req.StatePath)
	if err != nil {
		return providerRunResult{}, err
	}
	result, err := agentprovider.Run(ctx, agentprovider.Request{
		TargetName: req.Spec.Name,
		Stdout:     req.Stdout,
		Stderr:     req.Stderr,
		Provider:   body.Provider,
		Branch:     body.Git.Branch,
		Workspace:  workspace,
		Artifacts:  agentprovider.PromptArtifacts{PromptPath: artifacts.PromptPath},
		Env:        providerEnv,
		Attempt:    attempt,
		AttemptDir: attemptDir,
		Previous:   providerPreviousAttempt(previous),
		Now:        req.Now,
	})
	if statusErr := validateProjectCheckoutSnapshot(
		ctx,
		req.WorkDir,
		beforeProject,
	); statusErr != nil {
		if err != nil {
			return providerRunResult{Artifacts: result.Artifacts}, fmt.Errorf(
				"%w; %w",
				err,
				statusErr,
			)
		}
		return providerRunResult{Artifacts: result.Artifacts}, statusErr
	}
	if evidenceErr := validateTrustedEvidenceUnchanged(
		req.WorkDir,
		req.StatePath,
		beforeEvidence,
	); evidenceErr != nil {
		if err != nil {
			return providerRunResult{Artifacts: result.Artifacts}, fmt.Errorf(
				"%w; %w",
				err,
				evidenceErr,
			)
		}
		return providerRunResult{Artifacts: result.Artifacts}, evidenceErr
	}
	return providerRunResult{Artifacts: result.Artifacts}, err
}

func agentProviderEnv(
	req ExecuteRequest,
	body model.AgentSpec,
	workspace string,
	artifacts agentArtifacts,
	attempt int,
	attempts int,
	attemptDir string,
	feedbackPath string,
) map[string]string {
	providerEnv := copyEnv(req.Env)
	providerEnv["BACH_AGENT_ATTEMPT"] = strconv.Itoa(attempt)
	providerEnv["BACH_AGENT_MAX_ATTEMPTS"] = strconv.Itoa(attempts)
	providerEnv["BACH_AGENT_FEEDBACK_BUNDLE"] = feedbackPath
	providerEnv["BACH_AGENT_ATTEMPT_DIRECTORY"] = attemptDir
	providerEnv["BACH_AGENT_CONTEXT_PATH"] = artifacts.ContextPath
	providerEnv["BACH_AGENT_REPORT_PATH"] = artifacts.ReportPath
	providerEnv["BACH_AGENT_WORKSPACE"] = workspace
	providerEnv["BACH_AGENT_PROMPT_PATH"] = artifacts.PromptPath
	providerEnv["BACH_AGENT_TARGET"] = req.Spec.Name
	providerEnv["BACH_AGENT_MODE"] = body.Mode
	providerEnv["BACH_AGENT_ROLE"] = body.Role
	providerEnv["BACH_PROJECT_ROOT"] = req.WorkDir
	return providerEnv
}

func providerPreviousAttempt(previous *agentAttemptReport) *agentprovider.PreviousAttempt {
	if previous == nil {
		return nil
	}
	return &agentprovider.PreviousAttempt{
		ProviderType:        previous.ProviderType,
		ProviderSessionID:   previous.ProviderSessionID,
		ProviderEventsPath:  previous.ProviderEventsPath,
		ProviderSessionPath: previous.ProviderSessionPath,
		FeedbackBundle:      previous.FeedbackBundle,
	}
}
