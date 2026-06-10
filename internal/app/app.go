package app

import (
	"context"
	"fmt"
	"io"

	"github.com/applause/bachkator/internal/cli"
	"github.com/applause/bachkator/internal/config"
	gitpkg "github.com/applause/bachkator/internal/git"
	"github.com/applause/bachkator/internal/graph"
	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/runner"
)

type App struct {
	version string
}

func New(version string) App {
	return App{version: version}
}

func (a App) Execute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	return cli.ExecuteWithDependencies(ctx, args, stdout, stderr, a.version, cli.Dependencies{
		LoadProject:     loadProject,
		RunTarget:       runTarget,
		ExplainTarget:   explainTarget,
		AffectedTargets: affectedTargets,
	})
}

func loadProject(path string, opts cli.LoadOptions) (*cli.Project, error) {
	project, err := config.LoadWithOptions(
		path,
		config.LoadOptions{Variables: opts.Variables, Profiles: opts.Profiles},
	)
	if err != nil {
		return nil, err
	}
	return cliProject(project), nil
}

func cliProject(project *config.Project) *cli.Project {
	out := &cli.Project{
		Backing:          project,
		DefaultTarget:    project.DefaultTarget,
		Root:             project.Root,
		StatePath:        project.StatePath,
		SelectedProfiles: append([]string(nil), project.SelectedProfiles...),
		Targets:          make(map[string]*cli.Target, len(project.Targets)),
		Aliases:          make(map[string]*model.Alias, len(project.Aliases)),
	}
	for name, alias := range project.Aliases {
		out.Aliases[name] = &model.Alias{
			Name:       alias.Name,
			Target:     alias.Target,
			Deprecated: alias.Deprecated,
		}
	}
	for name, target := range project.Targets {
		out.Targets[name] = &cli.Target{
			Name:       target.Name,
			DependsOn:  append([]string(nil), target.DependsOn...),
			Spec:       target.Spec(),
			RiskLabels: graph.TargetRiskLabels(config.RuntimeProject(project), name),
		}
	}
	return out
}

func runTarget(
	ctx context.Context,
	project *cli.Project,
	targetName string,
	opts cli.RunOptions,
	stdout io.Writer,
	stderr io.Writer,
) error {
	configProject, err := configProject(project)
	if err != nil {
		return err
	}
	runner := runner.Runner{
		DryRun:      opts.DryRun,
		PlanJSON:    opts.PlanJSON,
		Force:       opts.Force,
		Yes:         opts.Yes,
		EnvFile:     opts.EnvFile,
		LogOnly:     opts.LogOnly,
		Verbose:     opts.Verbose,
		Parallelism: opts.Parallelism,
		Stdout:      stdout,
		Stderr:      stderr,
	}
	return runner.Run(ctx, config.RuntimeProject(configProject), targetName)
}

func explainTarget(project *cli.Project, targetName string) (string, error) {
	configProject, err := configProject(project)
	if err != nil {
		return "", err
	}
	return graph.Explain(config.RuntimeProject(configProject), targetName)
}

func affectedTargets(
	ctx context.Context,
	project *cli.Project,
	paths []string,
) ([]cli.AffectedTarget, error) {
	configProject, err := configProject(project)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		paths = gitpkg.ChangedFiles(ctx, configProject.Root)
	}
	targets := graph.AffectedTargets(config.RuntimeProject(configProject), paths)
	out := make([]cli.AffectedTarget, 0, len(targets))
	for _, target := range targets {
		out = append(
			out,
			cli.AffectedTarget{
				Name:    target.Name,
				Matches: append([]string(nil), target.Matches...),
			},
		)
	}
	return out, nil
}

func configProject(project *cli.Project) (*config.Project, error) {
	configProject, ok := project.Backing.(*config.Project)
	if !ok || configProject == nil {
		return nil, fmt.Errorf("loaded project is not backed by config")
	}
	return configProject, nil
}
