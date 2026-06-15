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
	targetNames := args
	if len(targetNames) == 0 && project.DefaultTarget != "" {
		targetNames = []string{project.DefaultTarget}
	}
	if len(targetNames) == 0 {
		return fmt.Errorf("no target provided and no default target configured")
	}
	if opts.json && !opts.dryRun {
		return fmt.Errorf("-json is only supported with -dry-run")
	}
	canonicalNames := make([]string, 0, len(targetNames))
	seen := map[string]bool{}
	for _, targetName := range targetNames {
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
		}
		if seen[canonicalName] {
			continue
		}
		seen[canonicalName] = true
		canonicalNames = append(canonicalNames, canonicalName)
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
	return deps.RunTarget(ctx, project, canonicalNames, runOptions, stdout, stderr)
}
