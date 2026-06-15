package agentprovider

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
)

type genericCommandProvider struct{}

func (genericCommandProvider) Runnable(provider model.Provider) bool {
	return len(provider.Command) > 0
}

func (genericCommandProvider) BaseCommand(provider model.Provider) []string {
	return append([]string(nil), provider.Command...)
}

func (genericCommandProvider) Run(ctx context.Context, request Request) (Result, error) {
	command := runenv.ExpandSlice(request.Provider.Command, request.Env)
	command = append(command, request.Artifacts.PromptPath)
	if len(command) == 0 {
		return Result{}, fmt.Errorf("target %q provider command is empty", request.TargetName)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = request.Workspace
	cmd.Env = runenv.List(request.Env)
	cmd.Stdout = request.Stdout
	cmd.Stderr = request.Stderr
	return Result{Artifacts: Artifacts{Type: request.Provider.Type}}, cmd.Run()
}
