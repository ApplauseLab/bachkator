package agentprovider

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/model"
)

type Artifacts struct {
	Type            string
	SessionID       string
	EventsPath      string
	SessionPath     string
	SummaryPath     string
	ExecutedCommand []string
	WorkspaceDirty  bool
}

type Result struct {
	Artifacts Artifacts
}

type PromptArtifacts struct {
	PromptPath string
}

type PreviousAttempt struct {
	ProviderType        string
	ProviderSessionID   string
	ProviderEventsPath  string
	ProviderSessionPath string
	FeedbackBundle      string
}

type Request struct {
	TargetName string
	Stdout     io.Writer
	Stderr     io.Writer
	Provider   model.Provider
	Branch     string
	Workspace  string
	Artifacts  PromptArtifacts
	Env        map[string]string
	Attempt    int
	AttemptDir string
	Previous   *PreviousAttempt
	Now        clock.NowFunc
}

func (r Request) now() time.Time {
	return clock.UTC(r.Now)
}

type providerRunner interface {
	Runnable(model.Provider) bool
	BaseCommand(model.Provider) []string
	Run(context.Context, Request) (Result, error)
}

func Runnable(provider model.Provider) bool {
	runner, ok := runnerFor(provider)
	if !ok {
		return false
	}
	return runner.Runnable(provider)
}

func BaseCommand(provider model.Provider) []string {
	runner, ok := runnerFor(provider)
	if !ok {
		return nil
	}
	return runner.BaseCommand(provider)
}

func Run(ctx context.Context, request Request) (Result, error) {
	runner, ok := runnerFor(request.Provider)
	if !ok || !runner.Runnable(request.Provider) {
		return Result{}, fmt.Errorf(
			"target %q provider %q is not runnable",
			request.TargetName,
			request.Provider.Type,
		)
	}
	return runner.Run(ctx, request)
}

func runnerFor(provider model.Provider) (providerRunner, bool) {
	switch provider.Type {
	case "agent":
		return genericCommandProvider{}, true
	case "opencode":
		return openCodeProvider{}, true
	default:
		return nil, false
	}
}
