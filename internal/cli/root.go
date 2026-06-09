package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type ProjectLoader func(path string, opts LoadOptions) (*Project, error)

type RunTargetFunc func(context.Context, *Project, string, RunOptions, io.Writer, io.Writer) error
type ExplainTargetFunc func(*Project, string) (string, error)
type AffectedTargetsFunc func(context.Context, *Project, []string) ([]AffectedTarget, error)

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
	RunTarget       RunTargetFunc
	ExplainTarget   ExplainTargetFunc
	AffectedTargets AffectedTargetsFunc
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
	cmd.SetArgs(normalizeArgs(args))
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	bindFlags(cmd, opts)
	bindCommands(cmd, ctx, opts, stdout, stderr, deps.withDefaults())
	return cmd
}

func runRoot(cmd *cobra.Command, opts *options, stdout io.Writer, version string) error {
	if opts.version {
		_, err := fmt.Fprintf(stdout, "bach %s\n", version)
		return err
	}
	return cmd.Help()

}

func bindCommands(
	root *cobra.Command,
	ctx context.Context,
	opts *options,
	stdout io.Writer,
	stderr io.Writer,
	deps Dependencies,
) {
	for _, adapter := range commandAdapters() {
		adapter := adapter
		command := &cobra.Command{
			Use:   adapter.use,
			Short: adapter.short,
			Args:  cobra.ArbitraryArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				commandContext := commandContext{
					context: ctx,
					opts:    opts,
					stdout:  stdout,
					stderr:  stderr,
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
			},
		}
		root.AddCommand(command)
	}
}
