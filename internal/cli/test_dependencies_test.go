package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/applause/bachkator/internal/config"
	gitpkg "github.com/applause/bachkator/internal/git"
	"github.com/applause/bachkator/internal/graph"
	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/runner"
)

func init() {
	defaultDependenciesForExecute = testDependencies
}

func testDependencies() Dependencies {
	return Dependencies{
		LoadProject:     testLoadProject,
		RunTarget:       testRunTarget,
		ExplainTarget:   testExplainTarget,
		AffectedTargets: testAffectedTargets,
	}
}

func testLoadProject(path string, opts LoadOptions) (*Project, error) {
	project, err := config.LoadWithOptions(
		path,
		config.LoadOptions{Variables: opts.Variables, Profiles: opts.Profiles},
	)
	if err != nil {
		return nil, err
	}
	return testCLIProject(project), nil
}

func testCLIProject(project *config.Project) *Project {
	out := &Project{
		Backing:          project,
		DefaultTarget:    project.DefaultTarget,
		Root:             project.Root,
		StatePath:        project.StatePath,
		SelectedProfiles: append([]string(nil), project.SelectedProfiles...),
		Targets:          make(map[string]*Target, len(project.Targets)),
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
		out.Targets[name] = &Target{
			Name:       target.Name,
			DependsOn:  append([]string(nil), target.DependsOn...),
			Spec:       target.Spec(),
			RiskLabels: graph.TargetRiskLabels(config.RuntimeProject(project), name),
		}
	}
	return out
}

func testRunTarget(
	ctx context.Context,
	project *Project,
	targetName string,
	opts RunOptions,
	stdout io.Writer,
	stderr io.Writer,
) error {
	configProject, err := testConfigProject(project)
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

func testExplainTarget(project *Project, targetName string) (string, error) {
	configProject, err := testConfigProject(project)
	if err != nil {
		return "", err
	}
	return graph.Explain(config.RuntimeProject(configProject), targetName)
}

func testAffectedTargets(
	ctx context.Context,
	project *Project,
	paths []string,
) ([]AffectedTarget, error) {
	configProject, err := testConfigProject(project)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		paths = gitpkg.ChangedFiles(ctx, configProject.Root)
	}
	targets := graph.AffectedTargets(config.RuntimeProject(configProject), paths)
	out := make([]AffectedTarget, 0, len(targets))
	for _, target := range targets {
		out = append(
			out,
			AffectedTarget{Name: target.Name, Matches: append([]string(nil), target.Matches...)},
		)
	}
	return out, nil
}

func testConfigProject(project *Project) (*config.Project, error) {
	configProject, ok := project.Backing.(*config.Project)
	if !ok || configProject == nil {
		return nil, fmt.Errorf("loaded project is not backed by config")
	}
	return configProject, nil
}
