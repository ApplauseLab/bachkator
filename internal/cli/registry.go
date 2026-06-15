package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type commandRegistry struct {
	adapters []commandAdapter
}

func newCommandRegistry() *commandRegistry {
	return &commandRegistry{}
}

func (r *commandRegistry) Register(name string, adapter commandAdapter) error {
	if name == "" {
		return fmt.Errorf("command adapter has no name")
	}
	if adapter.subcommand == nil {
		if adapter.use == "" || adapter.short == "" {
			return fmt.Errorf("command adapter %q is incomplete", name)
		}
		if adapter.run == nil {
			return fmt.Errorf("command adapter %q has no run function", name)
		}
	}
	for _, existing := range r.adapters {
		if existing.name == name {
			return fmt.Errorf("command adapter %q already registered", name)
		}
	}
	adapter.name = name
	r.adapters = append(r.adapters, adapter)
	return nil
}

func (r *commandRegistry) Adapters() []commandAdapter {
	return append([]commandAdapter(nil), r.adapters...)
}

func builtinCommandRegistry() *commandRegistry {
	registry := newCommandRegistry()
	mustRegisterCommandAdapter(registry, "list", commandAdapter{
		use:          "list",
		short:        "List configured targets",
		needsProject: true,
		bindFlags:    bindListFlags,
		run: func(ctx commandContext, args []string) error {
			return runList(
				ctx.project,
				ctx.opts.verbose,
				ctx.opts.listAliases,
				ctx.opts.listGenerated,
				ctx.stdout,
			)
		},
	})
	mustRegisterCommandAdapter(registry, "runs", commandAdapter{
		needsProject: true,
		subcommand: func(deps Dependencies, opts *options, stdout, _ io.Writer, _ io.Reader) *cobra.Command {
			return newRunsCommand(deps, opts, stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "logs", commandAdapter{
		use:          "logs <run-id>",
		short:        "Show recorded target logs",
		needsProject: true,
		bindFlags:    bindLogsFlags,
		run: func(ctx commandContext, args []string) error {
			return runLogs(ctx.project, ctx.deps, ctx.opts, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "artifacts", commandAdapter{
		use:          "artifacts [run-id]",
		short:        "List recorded artifacts",
		needsProject: true,
		bindFlags:    bindRunsFlags,
		run: func(ctx commandContext, args []string) error {
			return runListArtifacts(ctx.project, ctx.deps, ctx.opts, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "affected", commandAdapter{
		use:          "affected [path ...]",
		short:        "List targets affected by changed paths",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runAffected(ctx.context, ctx.project, ctx.deps, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "provenance", commandAdapter{
		use:          "provenance <path> [path ...]",
		short:        "Explain which targets generate or consume paths",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runProvenance(ctx.project, ctx.deps, ctx.opts.json, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "explain", commandAdapter{
		use:          "explain <target>",
		short:        "Explain a target or alias",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runExplain(ctx.project, ctx.deps, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "graph", commandAdapter{
		use:          "graph",
		short:        "Print the target graph",
		needsProject: true,
		bindFlags:    bindGraphFlags,
		run: func(ctx commandContext, args []string) error {
			return runGraph(ctx.project, ctx.deps, ctx.opts.graphFormat, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "quality", commandAdapter{
		use:          "quality <target>",
		short:        "Show quality reports and gates for a target",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runQuality(ctx.project, ctx.deps, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "report", commandAdapter{
		needsProject: false,
		subcommand: func(_ Dependencies, _ *options, stdout, _ io.Writer, stdin io.Reader) *cobra.Command {
			return newReportCommand(stdin, stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "validate", commandAdapter{
		use:          "validate",
		short:        "Validate the Bachfile without running targets",
		needsProject: false,
		run: func(ctx commandContext, args []string) error {
			return runValidate(ctx.deps, ctx.opts, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "reference", commandAdapter{
		use:          "reference [topic]",
		short:        "Show embedded reference documentation",
		needsProject: false,
		run: func(ctx commandContext, args []string) error {
			return runReference(args, ctx.stdout, ctx.stderr)
		},
	})
	mustRegisterCommandAdapter(registry, "init", commandAdapter{
		use:          "init [--provider opencode]",
		short:        "Create starter Bach adoption files",
		needsProject: false,
		bindFlags:    bindInitFlags,
		run: func(ctx commandContext, args []string) error {
			return runInit(ctx.context, ctx.deps, ctx.opts, args, ctx.stdout, ctx.stderr)
		},
	})
	mustRegisterCommandAdapter(registry, "factory", commandAdapter{
		needsProject: true,
		subcommand: func(deps Dependencies, opts *options, stdout, stderr io.Writer, _ io.Reader) *cobra.Command {
			return newFactoryCommand(deps, opts, stdout, stderr)
		},
	})
	mustRegisterCommandAdapter(registry, "plan", commandAdapter{
		needsProject: true,
		subcommand: func(deps Dependencies, opts *options, stdout, stderr io.Writer, _ io.Reader) *cobra.Command {
			return newPlanCommand(deps, opts, stdout, stderr)
		},
	})
	mustRegisterCommandAdapter(registry, "backend", commandAdapter{
		needsProject: false,
		subcommand: func(_ Dependencies, _ *options, stdout, _ io.Writer, _ io.Reader) *cobra.Command {
			return newBackendCommand(stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "run", commandAdapter{
		use:          "run <target> [target ...]",
		short:        "Run one or more targets",
		needsProject: true,
		bindFlags:    bindExecutionFlags,
		run: func(ctx commandContext, args []string) error {
			return runTarget(
				ctx.context,
				ctx.project,
				ctx.deps,
				ctx.opts,
				args,
				ctx.stdout,
				ctx.stderr,
			)
		},
	})
	return registry
}

func mustRegisterCommandAdapter(registry *commandRegistry, name string, adapter commandAdapter) {
	if err := registry.Register(name, adapter); err != nil {
		panic(err)
	}
}
