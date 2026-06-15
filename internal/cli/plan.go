package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/planbatch"
	"github.com/applauselab/bachkator/internal/planstatus"
	"github.com/spf13/cobra"
)

func newPlanCommand(
	deps Dependencies,
	opts *options,
	stdout io.Writer,
	stderr io.Writer,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Inspect or implement Markdown Plans",
	}
	cmd.AddCommand(newPlanStatusCommand(deps, opts, stdout))
	cmd.AddCommand(newPlanImplementCommand(deps, opts, stdout, stderr))
	cmd.AddCommand(newPlanReviewCommand(deps, opts, stdout))
	return cmd
}

func newPlanImplementCommand(
	deps Dependencies,
	opts *options,
	stdout io.Writer,
	stderr io.Writer,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "implement <plan-file> [plan-file ...]",
		Short: "Implement one or more Markdown Plans",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanImplement(cmd.Context(), deps, opts, args, stdout, stderr)
		},
	}
	bindPlanExecutionFlags(cmd, opts)
	return cmd
}

func newPlanReviewCommand(
	deps Dependencies,
	opts *options,
	stdout io.Writer,
) *cobra.Command {
	return &cobra.Command{
		Use:   "review <plan-file> [plan-file ...]",
		Short: "Review batch Plan outcomes by decision state",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanReview(cmd.Context(), deps, opts, args, stdout)
		},
	}
}

func newPlanStatusCommand(
	deps Dependencies,
	opts *options,
	stdout io.Writer,
) *cobra.Command {
	return &cobra.Command{
		Use:   "status <plan-file> [plan-file ...]",
		Short: "Inspect Markdown Plan status",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanStatus(cmd.Context(), deps, opts, args, stdout)
		},
	}
}

func runPlanImplement(
	ctx context.Context,
	deps Dependencies,
	opts *options,
	paths []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if len(paths) == 0 {
		return UsageErrorf("usage: bach plan implement <plan-file> [plan-file ...]")
	}
	if len(paths) == 1 && deps.PlanImplement != nil {
		return runSinglePlanImplement(ctx, deps, opts, paths[0], stdout, stderr)
	}
	if deps.PlanBatch == nil {
		return fmt.Errorf("plan batch dependency is not configured")
	}

	stopOn := planbatch.StopMode(opts.planStopOn)
	if !stopOn.Valid() {
		return UsageErrorf("invalid --stop-on value %q", opts.planStopOn)
	}

	out := stdout
	if opts.json {
		stdout = io.Discard
		stderr = io.Discard
	}

	result, err := deps.PlanBatch(ctx, projectFromContext(ctx), PlanBatchOptions{
		Paths:       paths,
		Parallelism: opts.planParallelism,
		StopOn:      stopOn,
		Force:       opts.force,
		DryRun:      opts.dryRun,
		Yes:         opts.yes,
		EnvFile:     opts.envFile,
		LogOnly:     opts.logOnly,
		Verbose:     opts.verbose,
		Stdout:      stdout,
		Stderr:      stderr,
	})
	if opts.json {
		if writeErr := writePlanBatchJSON(out, result); writeErr != nil {
			return writeErr
		}
	} else if writeErr := writePlanBatchHuman(out, result); writeErr != nil {
		return writeErr
	}
	return err
}

func runSinglePlanImplement(
	ctx context.Context,
	deps Dependencies,
	opts *options,
	path string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if deps.PlanImplement == nil {
		return fmt.Errorf("plan implement dependency is not configured")
	}
	out := stdout
	if opts.json {
		stdout = io.Discard
		stderr = io.Discard
	}
	result, err := deps.PlanImplement(ctx, projectFromContext(ctx), PlanImplementOptions{
		Path:        path,
		DryRun:      opts.dryRun,
		Force:       opts.force,
		Yes:         opts.yes,
		EnvFile:     opts.envFile,
		LogOnly:     opts.logOnly,
		Verbose:     opts.verbose,
		Parallelism: opts.jobs,
		Stdout:      stdout,
		Stderr:      stderr,
	})
	if opts.json {
		if writeErr := writePlanImplementJSON(out, result); writeErr != nil {
			return writeErr
		}
	} else if writeErr := writePlanImplementHuman(out, result); writeErr != nil {
		return writeErr
	}
	return err
}

func runPlanReview(
	ctx context.Context,
	deps Dependencies,
	opts *options,
	paths []string,
	stdout io.Writer,
) error {
	if deps.PlanReview == nil {
		return fmt.Errorf("plan review dependency is not configured")
	}
	result, err := deps.PlanReview(ctx, projectFromContext(ctx), PlanReviewOptions{Paths: paths})
	if err != nil {
		if errors.Is(err, bacherr.ErrValidationFailed) ||
			errors.Is(err, planstatus.ErrNoPlanPaths) {
			return UsageErrorf("usage: bach plan review <plan-file> [plan-file ...]")
		}
		return err
	}
	if opts.json {
		if err := writePlanReviewJSON(stdout, result); err != nil {
			return err
		}
	} else if err := writePlanReviewHuman(stdout, result); err != nil {
		return err
	}
	return nil
}

func runPlanStatus(
	ctx context.Context,
	deps Dependencies,
	opts *options,
	paths []string,
	stdout io.Writer,
) error {
	if deps.PlanStatus == nil {
		return fmt.Errorf("plan status dependency is not configured")
	}
	result, err := deps.PlanStatus(ctx, projectFromContext(ctx), paths)
	if err != nil {
		if errors.Is(err, bacherr.ErrValidationFailed) ||
			errors.Is(err, planstatus.ErrNoPlanPaths) {
			return UsageErrorf("usage: bach plan status <plan-file> [plan-file ...]")
		}
		return err
	}
	if opts.json {
		if err := writePlanStatusJSON(stdout, result); err != nil {
			return err
		}
	} else if err := writePlanStatusHuman(stdout, result); err != nil {
		return err
	}
	if hasErrorDiagnostics(result.Diagnostics) {
		return fmt.Errorf("plan status has validation errors")
	}
	return nil
}
