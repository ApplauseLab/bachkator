package app

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/applauselab/bachkator/internal/cli"
	"github.com/applauselab/bachkator/internal/config"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/graph"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
	"github.com/applauselab/bachkator/internal/query"
	"github.com/applauselab/bachkator/internal/runner"
	statestore "github.com/applauselab/bachkator/internal/state"
	targetpkg "github.com/applauselab/bachkator/internal/target"
)

type App struct {
	version        string
	targetHandlers runner.TargetHandlers
	qualityParsers quality.ReportParsers
	qualityGates   quality.GateEvaluators
	projects       *projectRegistry
}

type projectRegistry struct {
	mu       sync.RWMutex
	projects map[*cli.Project]*config.Project
}

func New(version string) App {
	return App{
		version:        version,
		targetHandlers: targetpkg.BuiltinTargetRegistry(),
		qualityParsers: quality.BuiltinReportParserRegistry(),
		qualityGates:   quality.BuiltinGateRegistry(),
		projects:       &projectRegistry{projects: map[*cli.Project]*config.Project{}},
	}
}

func (a App) Execute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	return cli.ExecuteWithDependencies(ctx, args, stdout, stderr, a.version, cli.Dependencies{
		LoadProject:     a.loadProject,
		ValidateProject: validateProject,
		RunTarget:       a.runTarget,
		ExplainTarget:   a.explainTarget,
		AffectedTargets: a.affectedTargets,
		Provenance:      a.provenance,
		GraphDocument:   a.graphDocument,
		Quality:         cli.QualityQueryFunc(qualitySnapshot),
		InspectRun:      inspectRun,
		ListRuns:        listRuns,
		ListArtifacts:   listArtifacts,
		Logs:            logs,
		InitProject:     initProject,
		FactorySubmit:   a.submitFactory,
		FactoryList:     a.listFactory,
		FactoryInspect:  a.inspectFactory,
		FactoryCancel:   a.cancelFactory,
		FactoryApprove:  a.approveFactory,
		FactoryStart:    a.startFactory,
		FactoryStatus:   a.statusFactory,
		PlanStatus:      a.planStatus,
		PlanImplement:   a.planImplement,
		PlanBatch:       a.planBatch,
		PlanReview:      a.planReview,
	})
}

func qualitySnapshot(
	project *cli.Project,
	limits query.QualityLimits,
) cli.QResult {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return cli.QResult{Err: err}
	}
	defer func() { _ = store.Close() }()
	snapshot, err := query.Quality(store, limits)
	return cli.QResult{Snapshot: snapshot, Err: err}
}

func inspectRun(project *cli.Project, opts query.RunInspectOptions) (query.RunInspection, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return query.RunInspection{}, err
	}
	defer func() { _ = store.Close() }()
	return query.InspectRun(store, opts)
}

func listRuns(project *cli.Project, opts query.RunListOptions) ([]query.RunListRecord, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return query.ListRuns(store, opts)
}

func listArtifacts(
	project *cli.Project,
	opts query.ArtifactListOptions,
) ([]query.ArtifactListRecord, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return query.ListArtifacts(store, opts)
}

func logs(project *cli.Project, opts query.LogOptions) ([]query.LogSection, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return query.Logs(store, opts)
}

func (a App) loadProject(path string, opts cli.LoadOptions) (*cli.Project, error) {
	project, err := config.LoadWithOptions(
		path,
		config.LoadOptions{Variables: opts.Variables, Profiles: opts.Profiles},
	)
	if err != nil {
		return nil, err
	}
	cliProject := cliProject(project)
	a.projects.store(cliProject, project)
	return cliProject, nil
}

func validateProject(path string, opts cli.LoadOptions) cli.ValidationReport {
	report := config.ValidateWithOptions(
		path,
		config.LoadOptions{Variables: opts.Variables, Profiles: opts.Profiles},
	)
	out := cli.ValidationReport{
		Valid: report.Valid,
		Files: append([]string(nil), report.Files...),
		Summary: cli.ValidationSummary{
			Targets:  report.Summary.Targets,
			Aliases:  report.Summary.Aliases,
			Inputs:   report.Summary.Inputs,
			Profiles: report.Summary.Profiles,
		},
		Diagnostics: make([]cli.ValidationDiagnostic, 0, len(report.Diagnostics)),
	}
	for _, diag := range report.Diagnostics {
		out.Diagnostics = append(out.Diagnostics, cli.ValidationDiagnostic{
			Severity: diag.Severity,
			File:     diag.File,
			Range: cli.DiagnosticRange{
				Start: cli.DiagnosticPosition{
					Line:   diag.Range.Start.Line,
					Column: diag.Range.Start.Column,
				},
				End: cli.DiagnosticPosition{
					Line:   diag.Range.End.Line,
					Column: diag.Range.End.Column,
				},
			},
			Message: diag.Message,
			Code:    diag.Code,
		})
	}
	return out
}

func cliProject(project *config.Project) *cli.Project {
	out := &cli.Project{
		DefaultTarget:    project.DefaultTarget,
		Root:             project.Root,
		StatePath:        project.StatePath,
		SelectedProfiles: append([]string(nil), project.SelectedProfiles...),
		Prompts:          clonePrompts(project.Prompts),
		Targets:          make(map[string]*cli.Target, len(project.Targets)),
		Aliases:          make(map[string]*model.Alias, len(project.Aliases)),
		Policies:         make(map[string]*cli.Policy, len(project.Policies)),
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
	for address, policy := range project.Policies {
		out.Policies[address] = &cli.Policy{
			Name:             policy.Name,
			Address:          address,
			Subject:          policy.Subject,
			SubjectWorkspace: policy.SubjectWorkspace,
			SubjectCommit:    policy.SubjectCommit,
			RequiredTargets:  append([]string(nil), policy.RequiredTargets...),
		}
	}
	return out
}

func clonePrompts(prompts map[string]*config.Prompt) map[string]*model.Prompt {
	if len(prompts) == 0 {
		return nil
	}
	out := make(map[string]*model.Prompt, len(prompts))
	for key, prompt := range prompts {
		if prompt == nil {
			continue
		}
		out[key] = &model.Prompt{
			Name:        prompt.Name,
			Path:        prompt.Path,
			Description: prompt.Description,
			Version:     prompt.Version,
		}
	}
	return out
}

func (r *projectRegistry) store(cliProject *cli.Project, configProject *config.Project) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projects[cliProject] = configProject
}

func (r *projectRegistry) configProject(cliProject *cli.Project) (*config.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	configProject := r.projects[cliProject]
	if configProject == nil {
		return nil, fmt.Errorf("loaded project is not registered with app")
	}
	return configProject, nil
}

func (a App) runTarget(
	ctx context.Context,
	project *cli.Project,
	targetNames []string,
	opts cli.RunOptions,
	stdout io.Writer,
	stderr io.Writer,
) error {
	configProject, err := a.configProject(project)
	if err != nil {
		return err
	}
	policyNames, normalTargetNames := splitPolicyTargets(configProject, targetNames)
	if len(policyNames) > 0 {
		if len(normalTargetNames) > 0 {
			return fmt.Errorf("policy nodes must be run separately from normal targets")
		}
		return runPolicyTargets(
			ctx,
			configProject,
			policyNames,
			opts,
			stdout,
			stderr,
			a.targetHandlers,
			a.qualityParsers,
			a.qualityGates,
		)
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
		Targets:     a.targetHandlers,
		Parsers:     a.qualityParsers,
		Gates:       a.qualityGates,
	}
	return runner.RunTargets(ctx, config.RuntimeProject(configProject), targetNames)
}

func (a App) explainTarget(project *cli.Project, targetName string) (graph.ExplainRecord, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return graph.ExplainRecord{}, err
	}
	if policy := configProject.Policies[targetName]; policy != nil {
		return graph.ExplainRecord{
			Target:           targetName,
			GeneratedPolicy:  true,
			Subject:          policy.Subject,
			SubjectWorkspace: policy.SubjectWorkspace,
			SubjectCommit:    policy.SubjectCommit,
			RequiredTargets:  append([]string(nil), policy.RequiredTargets...),
		}, nil
	}
	return graph.ExplainRecordWithHandlers(
		config.RuntimeProject(configProject),
		targetName,
		a.targetHandlers,
	)
}

func (a App) graphDocument(project *cli.Project) (graph.GraphDocument, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return graph.GraphDocument{}, err
	}
	return graph.BuildDocument(config.RuntimeProject(configProject)), nil
}

func (a App) affectedTargets(
	ctx context.Context,
	project *cli.Project,
	paths []string,
) ([]cli.AffectedTarget, error) {
	configProject, err := a.configProject(project)
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

func (a App) provenance(project *cli.Project, paths []string) ([]cli.PathProvenance, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return nil, err
	}
	records := graph.ProvenanceWithHandlers(
		config.RuntimeProject(configProject),
		paths,
		a.targetHandlers,
	)
	out := make([]cli.PathProvenance, 0, len(records))
	for _, record := range records {
		out = append(out, cli.PathProvenance{
			Path:      record.Path,
			Generated: record.Generated,
			Source:    record.Source,
			Producers: provenanceTargets(record.Producers),
			Consumers: provenanceTargets(record.Consumers),
			Status:    record.Status,
			Reasons:   append([]string(nil), record.Reasons...),
		})
	}
	return out, nil
}

func provenanceTargets(targets []graph.ProvenanceTarget) []cli.ProvenanceTarget {
	out := make([]cli.ProvenanceTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, cli.ProvenanceTarget{
			Target:            target.Target,
			Operation:         target.Operation,
			RegenerateCommand: target.RegenerateCommand,
			Outputs:           append([]string(nil), target.Outputs...),
			Inputs:            append([]string(nil), target.Inputs...),
		})
	}
	return out
}

func (a App) configProject(project *cli.Project) (*config.Project, error) {
	return a.projects.configProject(project)
}
