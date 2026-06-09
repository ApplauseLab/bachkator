package cli

import (
	"context"
	"fmt"
	"io"
)

func runTarget(
	ctx context.Context,
	project *Project,
	deps Dependencies,
	opts *options,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	targetName := project.DefaultTarget
	if len(args) > 0 {
		targetName = args[0]
	}
	if targetName == "" {
		return fmt.Errorf("no target provided and no default target configured")
	}
	if opts.json && !opts.dryRun {
		return fmt.Errorf("-json is only supported with -dry-run")
	}
	canonicalName, alias := project.ResolveTargetName(targetName)
	if alias != nil {
		hint := alias.Deprecated
		if hint == "" {
			hint = fmt.Sprintf("Use %s.", alias.Target)
		}
		if _, err := fmt.Fprintf(
			stderr,
			"alias %q resolves to %q. %s\n",
			alias.Name,
			alias.Target,
			hint,
		); err != nil {
			return err
		}
		targetName = canonicalName
	}

	if deps.RunTarget == nil {
		return fmt.Errorf("run service is not configured")
	}
	runOptions := RunOptions{
		DryRun:      opts.dryRun,
		PlanJSON:    opts.json,
		Force:       opts.force,
		Yes:         opts.yes,
		EnvFile:     opts.envFile,
		LogOnly:     opts.logOnly,
		Verbose:     opts.verbose,
		Parallelism: opts.jobs,
	}
	return deps.RunTarget(ctx, project, targetName, runOptions, stdout, stderr)
}
