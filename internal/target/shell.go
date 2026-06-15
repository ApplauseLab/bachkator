package target

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
)

type shellHandler struct{}

func (shellHandler) Type() model.TargetType { return model.TargetTypeShell }

func (shellHandler) Runnable(spec model.TargetSpec) bool {
	body, ok := spec.Body.(model.ShellSpec)
	return ok && (body.Shell != "" || len(body.Command) > 0)
}

func (shellHandler) Describe(_ context.Context, req DescribeRequest) (RunDescription, error) {
	body, ok := req.Spec.Body.(model.ShellSpec)
	if !ok {
		return RunDescription{}, fmt.Errorf(
			"target %q has %s body, want shell",
			req.Spec.Name,
			req.Spec.TargetType(),
		)
	}
	if body.Shell != "" {
		return RunDescription{
			Operation: runenv.Expand(body.Shell, req.Env),
			WorkDir:   body.WorkDir,
		}, nil
	}
	return RunDescription{
		Operation: strings.Join(runenv.ExpandSlice(body.Command, req.Env), " "),
		WorkDir:   body.WorkDir,
	}, nil
}

func (h shellHandler) Execute(ctx context.Context, req ExecuteRequest) error {
	commands, err := h.commands(ctx, req.Spec, req.Env)
	if err != nil {
		return err
	}
	for _, cmd := range commands {
		cmd.Dir = req.WorkDir
		cmd.Env = runenv.List(req.Env)
		cmd.Stdout = req.Stdout
		cmd.Stderr = req.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (shellHandler) FingerprintParts(body model.TargetBody) map[string]string {
	shell, _ := body.(model.ShellSpec)
	return map[string]string{
		"command": strings.Join(shell.Command, "\x00"),
		"shell":   shell.Shell,
		"workdir": shell.WorkDir,
	}
}

func (shellHandler) CompositeChildren(model.TargetBody) []CompositeChild { return nil }

func (shellHandler) commands(
	ctx context.Context,
	spec model.TargetSpec,
	env map[string]string,
) ([]*exec.Cmd, error) {
	body, ok := spec.Body.(model.ShellSpec)
	if !ok {
		return nil, fmt.Errorf("target %q has %s body, want shell", spec.Name, spec.TargetType())
	}
	if body.Shell != "" && len(body.Command) > 0 {
		return nil, fmt.Errorf("target %q must use command or shell, not both", spec.Name)
	}
	if body.Shell != "" {
		return []*exec.Cmd{
			exec.CommandContext(ctx, "/bin/sh", "-c", runenv.Expand(body.Shell, env)),
		}, nil
	}
	if len(body.Command) == 0 {
		return nil, fmt.Errorf("target %q has no command", spec.Name)
	}
	command := runenv.ExpandSlice(body.Command, env)
	return []*exec.Cmd{exec.CommandContext(ctx, command[0], command[1:]...)}, nil
}
