package cli

import "fmt"

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
	if adapter.use == "" || adapter.short == "" || adapter.run == nil {
		return fmt.Errorf("command adapter %q is incomplete", name)
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
		run: func(ctx commandContext, args []string) error {
			return runList(ctx.project, ctx.opts.verbose, ctx.opts.listAliases, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "runs", commandAdapter{
		use:          "runs [inspect <run-id>]",
		short:        "List recorded runs",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runRuns(ctx.project, ctx.opts, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "logs", commandAdapter{
		use:          "logs <run-id>",
		short:        "Show recorded target logs",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runLogs(ctx.project, ctx.opts, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "artifacts", commandAdapter{
		use:          "artifacts [run-id]",
		short:        "List recorded artifacts",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runListArtifacts(ctx.project, ctx.opts, args, ctx.stdout)
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
		run: func(ctx commandContext, args []string) error {
			return runGraph(ctx.project, ctx.opts.graphFormat, args, ctx.stdout)
		},
	})
	mustRegisterCommandAdapter(registry, "quality", commandAdapter{
		use:          "quality <target>",
		short:        "Show quality reports and gates for a target",
		needsProject: true,
		run: func(ctx commandContext, args []string) error {
			return runQuality(ctx.project, args, ctx.stdout)
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
	mustRegisterCommandAdapter(registry, "run", commandAdapter{
		use:          "run <target>",
		short:        "Run a target",
		needsProject: true,
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
