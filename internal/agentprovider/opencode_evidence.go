package agentprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/applauselab/bachkator/internal/evidence"
	gitpkg "github.com/applauselab/bachkator/internal/git"
)

func validateOpenCodeResume(
	ctx context.Context,
	target string,
	workspace string,
	branch string,
	previous *PreviousAttempt,
) error {
	if previous == nil {
		return fmt.Errorf("opencode resume requires previous attempt evidence")
	}
	if previous.ProviderType != "opencode" {
		return fmt.Errorf("opencode resume requires previous opencode provider evidence")
	}
	if err := validateOpenCodePreviousAttempt(previous); err != nil {
		return err
	}
	session, err := readOpenCodePreviousSession(previous.ProviderSessionPath)
	if err != nil {
		return err
	}
	if session.Target != target {
		return fmt.Errorf("opencode resume target = %q, want %q", session.Target, target)
	}
	if !samePath(session.Workspace, workspace) {
		return fmt.Errorf("opencode resume workspace = %q, want %q", session.Workspace, workspace)
	}
	currentBranch, err := gitpkg.Branch(ctx, workspace)
	if err != nil {
		return fmt.Errorf("opencode resume workspace branch: %w", err)
	}
	if currentBranch != branch {
		return fmt.Errorf("opencode resume branch = %q, want %q", currentBranch, branch)
	}
	return nil
}

func validateOpenCodePreviousAttempt(previous *PreviousAttempt) error {
	if previous.ProviderSessionID == "" {
		return fmt.Errorf("opencode resume requires previous provider session ID")
	}
	if previous.ProviderSessionPath == "" {
		return fmt.Errorf("opencode resume requires previous provider session artifact")
	}
	if previous.ProviderEventsPath == "" {
		return fmt.Errorf("opencode resume requires previous provider event artifact")
	}
	if previous.FeedbackBundle == "" {
		return fmt.Errorf("opencode resume requires previous feedback bundle")
	}
	for label, path := range map[string]string{
		"provider session": previous.ProviderSessionPath,
		"provider events":  previous.ProviderEventsPath,
		"feedback bundle":  previous.FeedbackBundle,
	} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("opencode resume %s artifact %q: %w", label, path, err)
		}
	}
	return nil
}

type openCodePreviousSession struct {
	Target    string `json:"target"`
	Workspace string `json:"workspace"`
}

func readOpenCodePreviousSession(path string) (openCodePreviousSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return openCodePreviousSession{}, fmt.Errorf(
			"read previous opencode session artifact: %w",
			err,
		)
	}
	var session openCodePreviousSession
	if err := json.Unmarshal(data, &session); err != nil {
		return openCodePreviousSession{}, fmt.Errorf(
			"parse previous opencode session artifact: %w",
			err,
		)
	}
	return session, nil
}

func writeOpenCodeSession(
	target string,
	attempt int,
	workspace string,
	artifacts Artifacts,
	createdAt time.Time,
) error {
	value := map[string]any{
		"schema":           "bach.provider_session.v1",
		"provider":         "opencode",
		"session_id":       artifacts.SessionID,
		"target":           target,
		"attempt":          attempt,
		"workspace":        workspace,
		"workspace_dirty":  artifacts.WorkspaceDirty,
		"events_path":      artifacts.EventsPath,
		"executed_command": artifacts.ExecutedCommand,
		"created_at":       createdAt.UTC().Format(time.RFC3339),
	}
	return evidence.WriteJSONArtifact(artifacts.SessionPath, value)
}

func writeOpenCodeSummary(path string, summary opencodeEventSummary) error {
	value := map[string]any{
		"schema":     "bach.provider_summary.v1",
		"provider":   "opencode",
		"session_id": summary.SessionID,
	}
	if summary.FinishReason != "" {
		value["finish_reason"] = summary.FinishReason
	}
	if summary.Tokens != nil {
		value["tokens"] = summary.Tokens
	}
	if summary.Cost != nil {
		value["cost"] = summary.Cost
	}
	return evidence.WriteJSONArtifact(path, value)
}
