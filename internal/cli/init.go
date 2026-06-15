package cli

import (
	"context"
	"fmt"
	"io"
)

func runInit(
	ctx context.Context,
	deps Dependencies,
	opts *options,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if len(args) > 0 {
		return fmt.Errorf("init does not accept positional arguments")
	}
	if deps.InitProject == nil {
		return fmt.Errorf("init project dependency is not configured")
	}
	return deps.InitProject(ctx, InitOptions{
		ConfigPath: opts.configPath,
		Provider:   opts.initProvider,
		DryRun:     opts.dryRun,
	}, stdout, stderr)
}
