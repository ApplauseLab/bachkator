package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/applauselab/bachkator/internal/graph"
	"github.com/spf13/cobra"
)

type ProjectLoader func(path string, opts LoadOptions) (*Project, error)
type ProjectValidator func(path string, opts LoadOptions) ValidationReport

type RunTargetFunc func(context.Context, *Project, []string, RunOptions, io.Writer, io.Writer) error
type ExplainTargetFunc func(*Project, string) (graph.ExplainRecord, error)
type AffectedTargetsFunc func(context.Context, *Project, []string) ([]AffectedTarget, error)
type ProvenanceFunc func(*Project, []string) ([]PathProvenance, error)
type GraphDocumentFunc func(*Project) (graph.GraphDocument, error)
type InitProjectFunc func(context.Context, InitOptions, io.Writer, io.Writer) error

type InitOptions struct {
	ConfigPath string
	Provider   string
	DryRun     bool
}

type RunOptions struct {
	DryRun      bool
	PlanJSON    bool
	Force       bool
	Yes         bool
	EnvFile     string
	LogOnly     bool
	Verbose     bool
	Parallelism int
}

type Dependencies struct {
	LoadProject     ProjectLoader
	ValidateProject ProjectValidator
	RunTarget       RunTargetFunc
	ExplainTarget   ExplainTargetFunc
	AffectedTargets AffectedTargetsFunc
	Provenance      ProvenanceFunc
	GraphDocument   GraphDocumentFunc
	Quality         QualityQuerier
	InspectRun      InspectRunFunc
	ListRuns        ListRunsFunc
	ListArtifacts   ListArtifactsFunc
	Logs            LogsFunc
	InitProject     InitProjectFunc
	FactorySubmit   FactorySubmitFunc
	FactoryList     FactoryListFunc
	FactoryInspect  FactoryInspectFunc
	FactoryCancel   FactoryCancelFunc
	FactoryApprove  FactoryApproveFunc
	FactoryStart    FactoryStartFunc
	FactoryStatus   FactoryStatusFunc
	PlanStatus      PlanStatusFunc
	PlanImplement   PlanImplementFunc
	PlanBatch       PlanBatchFunc
	PlanReview      PlanReviewFunc
}

func DefaultDependencies() Dependencies {
	return Dependencies{}
}

func (deps Dependencies) withDefaults() Dependencies {
	return deps
}

var defaultDependenciesForExecute = DefaultDependencies

func Execute(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	version string,
) error {
	return ExecuteWithDependencies(
		ctx,
		args,
		stdout,
		stderr,
		version,
		defaultDependenciesForExecute(),
	)
}

func ExecuteWithDependencies(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	version string,
	deps Dependencies,
) error {
	cmd := NewRootCommand(ctx, args, stdout, stderr, version, deps)
	return cmd.Execute()
}

func NewRootCommand(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	version string,
	deps Dependencies,
) *cobra.Command {
	opts := defaultOptions()
	cmd := &cobra.Command{
		Use:           "bach <command>",
		Short:         "Bachkator project automation",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd, opts, stdout, version)
		},
	}
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetContext(ctx)
	bindFlags(cmd, opts)
	bindCommands(cmd, ctx, opts, stdout, stderr, os.Stdin, deps.withDefaults())
	return cmd
}

func runRoot(cmd *cobra.Command, opts *options, stdout io.Writer, version string) error {
	if opts.version {
		_, err := fmt.Fprintf(stdout, "bach %s\n", version)
		return err
	}
	return cmd.Help()

}

type projectContextKey struct{}

func withProject(ctx context.Context, project *Project) context.Context {
	return context.WithValue(ctx, projectContextKey{}, project)
}

func projectFromContext(ctx context.Context) *Project {
	project, _ := ctx.Value(projectContextKey{}).(*Project)
	return project
}

func bindCommands(
	root *cobra.Command,
	ctx context.Context,
	opts *options,
	stdout io.Writer,
	stderr io.Writer,
	stdin io.Reader,
	deps Dependencies,
) {
	for _, adapter := range commandAdapters() {
		if adapter.subcommand != nil {
			command := adapter.subcommand(deps, opts, stdout, stderr, stdin)
			if command == nil {
				continue
			}
			if adapter.needsProject {
				command.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
					if deps.LoadProject == nil {
						return fmt.Errorf("project loader is not configured")
					}
					project, err := deps.LoadProject(
						opts.configPath,
						LoadOptions{Variables: opts.variables, Profiles: opts.profiles},
					)
					if err != nil {
						return err
					}
					cmd.SetContext(withProject(cmd.Context(), project))
					return nil
				}
			}
			root.AddCommand(command)
			continue
		}

		command := &cobra.Command{
			Use:                adapter.use,
			Short:              adapter.short,
			Args:               cobra.ArbitraryArgs,
			DisableFlagParsing: adapter.disableFlagParsing,
		}
		if adapter.bindFlags != nil {
			adapter.bindFlags(command, opts)
		}
		command.RunE = func(cmd *cobra.Command, args []string) error {
			commandContext := commandContext{
				context: ctx,
				opts:    opts,
				stdout:  stdout,
				stderr:  stderr,
				stdin:   stdin,
			}
			if adapter.needsProject {
				if deps.LoadProject == nil {
					return fmt.Errorf("project loader is not configured")
				}
				project, err := deps.LoadProject(
					opts.configPath,
					LoadOptions{Variables: opts.variables, Profiles: opts.profiles},
				)
				if err != nil {
					return err
				}
				commandContext.project = project
			}
			commandContext.deps = deps
			return adapter.run(commandContext, args)
		}
		root.AddCommand(command)
	}
}
