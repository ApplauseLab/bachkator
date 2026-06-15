package agentprovider

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
)

const (
	maxOpenCodeRawEventBytes  = 10 * 1024 * 1024
	maxOpenCodeEventLineBytes = 1024 * 1024
	maxOpenCodeLogTextBytes   = 1024 * 1024
)

type openCodeProvider struct{}

type opencodeEventSummary struct {
	SessionID    string         `json:"session_id,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
	Tokens       map[string]any `json:"tokens,omitempty"`
	Cost         any            `json:"cost,omitempty"`
	TextBytes    int            `json:"-"`
}

func (openCodeProvider) Runnable(model.Provider) bool { return true }

func (openCodeProvider) BaseCommand(model.Provider) []string {
	return []string{"opencode", "run"}
}

func (openCodeProvider) Run(ctx context.Context, request Request) (Result, error) {
	command := []string{"opencode", "run", "--format", "json"}
	if request.Attempt > 1 {
		if err := validateOpenCodeResume(
			ctx,
			request.TargetName,
			request.Workspace,
			request.Branch,
			request.Previous,
		); err != nil {
			return Result{}, err
		}
		command = append(command, "--session", request.Previous.ProviderSessionID)
	}
	command = append(command, "Follow the attached Bach agent prompt.", "--file")
	command = append(command, request.Artifacts.PromptPath)
	eventsPath := filepath.Join(request.AttemptDir, "provider-events.raw.jsonl")
	sessionPath := filepath.Join(request.AttemptDir, "provider-session.json")
	summaryPath := filepath.Join(request.AttemptDir, "provider-summary.json")
	result := Result{Artifacts: Artifacts{
		Type:            "opencode",
		EventsPath:      eventsPath,
		SessionPath:     sessionPath,
		SummaryPath:     summaryPath,
		ExecutedCommand: append([]string(nil), command...),
		WorkspaceDirty:  workspaceDirty(ctx, request.Workspace),
	}}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = request.Workspace
	cmd.Env = runenv.List(request.Env)
	cmd.Stderr = request.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}
	if err := cmd.Start(); err != nil {
		return result, err
	}
	summary, parseErr := captureOpenCodeEvents(stdout, eventsPath, request.Stdout)
	runErr := cmd.Wait()
	result.Artifacts.SessionID = summary.SessionID
	if summary.SessionID != "" {
		if err := writeOpenCodeSession(
			request.TargetName,
			request.Attempt,
			request.Workspace,
			result.Artifacts,
			request.now(),
		); err != nil {
			return result, err
		}
	}
	if err := writeOpenCodeSummary(summaryPath, summary); err != nil {
		return result, err
	}
	if parseErr != nil {
		return result, parseErr
	}
	if runErr != nil {
		return result, runErr
	}
	if summary.SessionID == "" {
		return result, fmt.Errorf("opencode provider did not emit a sessionID")
	}
	return result, nil
}
