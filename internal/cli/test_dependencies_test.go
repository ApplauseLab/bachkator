package cli

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/internal/factorydaemon"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/graph"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/query"
	"github.com/applauselab/bachkator/internal/runner"
	statestore "github.com/applauselab/bachkator/internal/state"
)

func init() {
	defaultDependenciesForExecute = testDependencies
}

var testProjectRegistry = struct {
	sync.RWMutex
	projects map[*Project]*config.Project
}{projects: map[*Project]*config.Project{}}

func testDependencies() Dependencies {
	return Dependencies{
		LoadProject:     testLoadProject,
		ValidateProject: testValidateProject,
		RunTarget:       testRunTarget,
		ExplainTarget:   testExplainTarget,
		AffectedTargets: testAffectedTargets,
		Provenance:      testProvenance,
		GraphDocument:   testGraphDocument,
		Quality:         QualityQueryFunc(testQuality),
		InspectRun:      testInspectRun,
		ListRuns:        testListRuns,
		ListArtifacts:   testListArtifacts,
		Logs:            testLogs,
		InitProject:     testInitProject,
		FactorySubmit:   testFactorySubmit,
		FactoryList:     testFactoryList,
		FactoryInspect:  testFactoryInspect,
		FactoryCancel:   testFactoryCancel,
		FactoryStart:    testFactoryStart,
		FactoryStatus:   testFactoryStatus,
	}
}

func testInitProject(context.Context, InitOptions, io.Writer, io.Writer) error {
	return nil
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

func testValidateProject(path string, opts LoadOptions) ValidationReport {
	report := config.ValidateWithOptions(
		path,
		config.LoadOptions{Variables: opts.Variables, Profiles: opts.Profiles},
	)
	out := ValidationReport{
		Valid: report.Valid,
		Files: append([]string(nil), report.Files...),
		Summary: ValidationSummary{
			Targets:  report.Summary.Targets,
			Aliases:  report.Summary.Aliases,
			Inputs:   report.Summary.Inputs,
			Profiles: report.Summary.Profiles,
		},
		Diagnostics: make([]ValidationDiagnostic, 0, len(report.Diagnostics)),
	}
	for _, diag := range report.Diagnostics {
		out.Diagnostics = append(out.Diagnostics, ValidationDiagnostic{
			Severity: diag.Severity,
			File:     diag.File,
			Range: DiagnosticRange{
				Start: DiagnosticPosition{
					Line:   diag.Range.Start.Line,
					Column: diag.Range.Start.Column,
				},
				End: DiagnosticPosition{
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

func testCLIProject(project *config.Project) *Project {
	out := &Project{
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
	testProjectRegistry.Lock()
	testProjectRegistry.projects[out] = project
	testProjectRegistry.Unlock()
	return out
}

func testRunTarget(
	ctx context.Context,
	project *Project,
	targetNames []string,
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
	return runner.RunTargets(ctx, config.RuntimeProject(configProject), targetNames)
}

func testExplainTarget(project *Project, targetName string) (graph.ExplainRecord, error) {
	configProject, err := testConfigProject(project)
	if err != nil {
		return graph.ExplainRecord{}, err
	}
	return graph.BuildExplainRecord(config.RuntimeProject(configProject), targetName)
}

func testGraphDocument(project *Project) (graph.GraphDocument, error) {
	configProject, err := testConfigProject(project)
	if err != nil {
		return graph.GraphDocument{}, err
	}
	return graph.BuildDocument(config.RuntimeProject(configProject)), nil
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

func testProvenance(project *Project, paths []string) ([]PathProvenance, error) {
	configProject, err := testConfigProject(project)
	if err != nil {
		return nil, err
	}
	records := graph.Provenance(config.RuntimeProject(configProject), paths)
	out := make([]PathProvenance, 0, len(records))
	for _, path := range records {
		out = append(out, PathProvenance{
			Path:      path.Path,
			Generated: path.Generated,
			Source:    path.Source,
			Producers: testProvenanceTargets(path.Producers),
			Consumers: testProvenanceTargets(path.Consumers),
			Status:    path.Status,
			Reasons:   append([]string(nil), path.Reasons...),
		})
	}
	return out, nil
}

func testProvenanceTargets(targets []graph.ProvenanceTarget) []ProvenanceTarget {
	out := make([]ProvenanceTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, ProvenanceTarget{
			Target:            target.Target,
			Operation:         target.Operation,
			RegenerateCommand: target.RegenerateCommand,
			Outputs:           append([]string(nil), target.Outputs...),
			Inputs:            append([]string(nil), target.Inputs...),
		})
	}
	return out
}

func testQuality(project *Project, limits query.QualityLimits) QResult {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return QResult{Err: err}
	}
	defer func() { _ = store.Close() }()
	snapshot, err := query.Quality(store, limits)
	return QResult{Snapshot: snapshot, Err: err}
}

func testInspectRun(project *Project, opts query.RunInspectOptions) (query.RunInspection, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return query.RunInspection{}, err
	}
	defer func() { _ = store.Close() }()
	return query.InspectRun(store, opts)
}

func testListRuns(project *Project, opts query.RunListOptions) ([]query.RunListRecord, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return query.ListRuns(store, opts)
}

func testListArtifacts(
	project *Project,
	opts query.ArtifactListOptions,
) ([]query.ArtifactListRecord, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return query.ListArtifacts(store, opts)
}

func testLogs(project *Project, opts query.LogOptions) ([]query.LogSection, error) {
	store, err := statestore.NewStore(project.StatePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return query.Logs(store, opts)
}

func testConfigProject(project *Project) (*config.Project, error) {
	testProjectRegistry.RLock()
	defer testProjectRegistry.RUnlock()
	configProject := testProjectRegistry.projects[project]
	if configProject == nil {
		return nil, fmt.Errorf("loaded project is not registered with test config")
	}
	return configProject, nil
}

func testFactorySubmit(
	ctx context.Context,
	project *Project,
	factoryName string,
	opts FactorySubmitOptions,
) (factorypkg.SubmitResult, error) {
	configProject, factoryConfig, workflow, err := testResolveFactory(
		project,
		factoryName,
		opts.Workflow,
		true,
	)
	if err != nil {
		return factorypkg.SubmitResult{}, err
	}
	if !factoryConfig.ManualEnabled() {
		return factorypkg.SubmitResult{}, fmt.Errorf(
			"factory %q has no manual trigger",
			factoryName,
		)
	}
	return testFactoryService(configProject).Submit(ctx, factorypkg.SubmitOptions{
		Factory:           factoryConfig.Name,
		Workflow:          workflow,
		Title:             opts.Title,
		Body:              opts.Body,
		BodyFile:          opts.BodyFile,
		Priority:          opts.Priority,
		Labels:            opts.Labels,
		DedupeKey:         opts.DedupeKey,
		SubmittedPlanPath: opts.Plan,
	})
}

func testFactoryList(
	ctx context.Context,
	project *Project,
	factoryName string,
	opts FactoryListOptions,
) ([]factorypkg.WorkItem, error) {
	configProject, factoryConfig, workflow, err := testResolveFactory(
		project,
		factoryName,
		opts.Workflow,
		false,
	)
	if err != nil {
		return nil, err
	}
	return testFactoryService(configProject).List(ctx, factorypkg.ListOptions{
		Factory:  factoryConfig.Name,
		Workflow: workflow,
		Status:   opts.Status,
	})
}

func testFactoryInspect(
	ctx context.Context,
	project *Project,
	factoryName string,
	id string,
) (factorypkg.WorkItem, error) {
	configProject, factoryConfig, _, err := testResolveFactory(project, factoryName, "", false)
	if err != nil {
		return factorypkg.WorkItem{}, err
	}
	return testFactoryService(configProject).Get(ctx, factoryConfig.Name, id)
}

func testFactoryCancel(
	ctx context.Context,
	project *Project,
	factoryName string,
	id string,
	opts FactoryCancelOptions,
) (factorypkg.WorkItem, error) {
	configProject, factoryConfig, _, err := testResolveFactory(project, factoryName, "", false)
	if err != nil {
		return factorypkg.WorkItem{}, err
	}
	return testFactoryService(configProject).Cancel(ctx, factorypkg.CancelOptions{
		Factory: factoryConfig.Name,
		ID:      id,
		Reason:  opts.Reason,
	})
}

func testFactoryStart(
	ctx context.Context,
	_ *Project,
	_ string,
	_ FactoryStartOptions,
) (factorydaemon.StartResult, error) {
	<-ctx.Done()
	return factorydaemon.StartResult{DaemonID: "test-daemon"}, nil
}

func testFactoryStatus(
	ctx context.Context,
	project *Project,
	factoryName string,
) (factorydaemon.StatusResult, error) {
	configProject, factoryConfig, _, err := testResolveFactory(project, factoryName, "", false)
	if err != nil {
		return factorydaemon.StatusResult{}, err
	}
	runtimeProject := config.RuntimeProject(configProject)
	client := backend.NewProjectClient(
		configProject.Root,
		configProject.StatePath,
		runtimeProject.Backend,
	)
	status, err := client.Factory.DaemonStatus(ctx, factoryConfig.Name, statestoreNow())
	return factorydaemon.StatusResult{Status: status}, err
}

func testResolveFactory(
	project *Project,
	factoryName string,
	requestedWorkflow string,
	resolveWorkflow bool,
) (*config.Project, *config.Factory, string, error) {
	configProject, err := testConfigProject(project)
	if err != nil {
		return nil, nil, "", err
	}
	factoryConfig := configProject.Factories[factoryName]
	if factoryConfig == nil {
		return nil, nil, "", fmt.Errorf("unknown factory %q", factoryName)
	}
	if !resolveWorkflow && requestedWorkflow == "" {
		return configProject, factoryConfig, "", nil
	}
	workflow, err := factoryConfig.ResolveWorkflow(requestedWorkflow)
	if err != nil {
		return nil, nil, "", err
	}
	return configProject, factoryConfig, workflow, nil
}

func testFactoryService(project *config.Project) factorypkg.Service {
	runtimeProject := config.RuntimeProject(project)
	client := backend.NewProjectClient(project.Root, project.StatePath, runtimeProject.Backend)
	return factorypkg.Service{
		Root:  project.Root,
		Queue: factorypkg.BackendQueue{Client: &client.Factory},
	}
}

func statestoreNow() time.Time {
	return time.Now().UTC()
}
