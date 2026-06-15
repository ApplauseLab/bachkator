package app

import (
	"context"
	"io"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/cli"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/planbatch"
	"github.com/applauselab/bachkator/internal/planexecute"
	"github.com/applauselab/bachkator/internal/planstatus"
)

func (a App) planStatus(
	ctx context.Context,
	project *cli.Project,
	paths []string,
) (cli.PlanStatusResult, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return cli.PlanStatusResult{}, err
	}
	runtimeProject := config.RuntimeProject(configProject)
	client := backend.NewProjectClient(
		configProject.Root,
		configProject.StatePath,
		runtimeProject.Backend,
	)
	result, err := planstatus.Status(
		ctx,
		runtimeProject,
		client.Plans,
		planstatus.Options{Paths: paths},
	)
	if err != nil {
		return cli.PlanStatusResult{}, err
	}
	return cli.PlanStatusResult{
		Records:     result.Records,
		Waves:       result.Selection.Waves,
		Diagnostics: result.Diagnostics,
	}, nil
}

func (a App) planImplement(
	ctx context.Context,
	project *cli.Project,
	opts cli.PlanImplementOptions,
) (planexecute.Result, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return planexecute.Result{}, err
	}
	runtimeProject := config.RuntimeProject(configProject)
	client := backend.NewProjectClient(
		configProject.Root,
		configProject.StatePath,
		runtimeProject.Backend,
	)
	service := planexecute.Service{
		Project: runtimeProject,
		Backend: client,
		Targets: a.targetHandlers,
		Parsers: a.qualityParsers,
		Gates:   a.qualityGates,
		Stdout:  writerOrDiscard(opts.Stdout),
		Stderr:  writerOrDiscard(opts.Stderr),
	}
	return service.Implement(ctx, planexecute.Options{
		Path:        opts.Path,
		DryRun:      opts.DryRun,
		Force:       opts.Force,
		Yes:         opts.Yes,
		EnvFile:     opts.EnvFile,
		LogOnly:     opts.LogOnly,
		Verbose:     opts.Verbose,
		Parallelism: opts.Parallelism,
	})
}

func (a App) planBatch(
	ctx context.Context,
	project *cli.Project,
	opts cli.PlanBatchOptions,
) (planbatch.Result, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return planbatch.Result{}, err
	}
	runtimeProject := config.RuntimeProject(configProject)
	client := backend.NewProjectClient(
		configProject.Root,
		configProject.StatePath,
		runtimeProject.Backend,
	)

	exec := planexecute.Service{
		Project: runtimeProject,
		Backend: client,
		Targets: a.targetHandlers,
		Parsers: a.qualityParsers,
		Gates:   a.qualityGates,
		Stdout:  writerOrDiscard(opts.Stdout),
		Stderr:  writerOrDiscard(opts.Stderr),
	}

	batch := planbatch.Service{Implement: exec.Implement}
	return batch.Execute(ctx, runtimeProject, client.Plans, planbatch.Options{
		Paths:       opts.Paths,
		Parallelism: opts.Parallelism,
		StopOn:      opts.StopOn,
		Force:       opts.Force,
		DryRun:      opts.DryRun,
		Yes:         opts.Yes,
		EnvFile:     opts.EnvFile,
		LogOnly:     opts.LogOnly,
		Verbose:     opts.Verbose,
		Template:    opts.Template,
	})
}

func (a App) planReview(
	ctx context.Context,
	project *cli.Project,
	opts cli.PlanReviewOptions,
) (cli.PlanReviewResult, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return cli.PlanReviewResult{}, err
	}
	runtimeProject := config.RuntimeProject(configProject)
	client := backend.NewProjectClient(
		configProject.Root,
		configProject.StatePath,
		runtimeProject.Backend,
	)
	result, err := planstatus.Status(
		ctx,
		runtimeProject,
		client.Plans,
		planstatus.Options{Paths: opts.Paths},
	)
	if err != nil {
		return cli.PlanReviewResult{}, err
	}
	return cli.PlanReviewResult{
		Queue:       planbatch.ReviewStatus(result.Records),
		Diagnostics: result.Diagnostics,
	}, nil
}

func writerOrDiscard(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}
	return writer
}
