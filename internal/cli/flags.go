package cli

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type options struct {
	configPath   string
	variables    variableFlags
	profiles     []string
	listAliases  bool
	verbose      bool
	runsLimit    int
	runsTarget   string
	runsStatus   string
	runsSince    string
	artifactPath string
	logsFailed   bool
	logsLast     int
	logsErrors   bool
	dryRun       bool
	force        bool
	yes          bool
	envFile      string
	logOnly      bool
	jobs         int
	json         bool
	version      bool
	graphFormat  string
}

func defaultOptions() *options {
	return &options{
		configPath:  "Bachfile",
		variables:   variableFlags{},
		runsLimit:   10,
		jobs:        runtime.NumCPU(),
		graphFormat: "mermaid",
	}
}

func bindFlags(cmd *cobra.Command, opts *options) {
	cmd.PersistentFlags().
		StringVarP(&opts.configPath, "file", "f", opts.configPath, "HCL build file to use")
	cmd.PersistentFlags().
		Var(&opts.variables, "var", "set variable value as name=value; may be repeated")
	cmd.PersistentFlags().
		StringArrayVar(&opts.profiles, "profile", nil, "select environment profile; may be repeated with later profiles winning")
	cmd.PersistentFlags().
		BoolVar(&opts.listAliases, "aliases", false, "include target aliases with list")
	cmd.PersistentFlags().
		BoolVar(&opts.verbose, "verbose", false, "show extra target metadata and stream output for quiet targets")
	cmd.PersistentFlags().
		IntVar(&opts.runsLimit, "runs-limit", opts.runsLimit, "maximum runs to list; use 0 for all")
	cmd.PersistentFlags().
		StringVar(&opts.runsTarget, "target", "", "filter runs or artifacts by target")
	cmd.PersistentFlags().
		StringVar(&opts.runsStatus, "status", "", "filter runs or artifacts by run status")
	cmd.PersistentFlags().
		StringVar(&opts.runsSince, "since", "", "filter runs or artifacts since a duration like 24h or an RFC3339 time")
	cmd.PersistentFlags().
		StringVar(&opts.artifactPath, "artifact", "", "filter runs or artifacts by artifact path substring")
	cmd.PersistentFlags().BoolVar(&opts.logsFailed, "failed", false, "show only failed target logs")
	cmd.PersistentFlags().IntVar(&opts.logsLast, "last", 0, "show only the last N log lines")
	cmd.PersistentFlags().BoolVar(&opts.logsErrors, "errors", false, "show only likely error lines")
	cmd.PersistentFlags().
		BoolVar(&opts.dryRun, "dry-run", false, "print commands without running them")
	cmd.PersistentFlags().
		BoolVar(&opts.force, "force", false, "run targets even when cached state is fresh")
	cmd.PersistentFlags().
		BoolVar(&opts.yes, "yes", false, "confirm execution of targets marked requires_confirmation")
	cmd.PersistentFlags().
		StringVar(&opts.envFile, "env-file", "", "load command environment from file instead of project .env")
	cmd.PersistentFlags().
		BoolVar(&opts.logOnly, "log-only", false, "suppress command output; keep progress on terminal and logs")
	cmd.PersistentFlags().
		IntVarP(&opts.jobs, "jobs", "j", opts.jobs, "maximum targets to run in parallel")
	cmd.PersistentFlags().
		BoolVar(&opts.json, "json", false, "print machine-readable output when combined with --dry-run")
	cmd.PersistentFlags().BoolVar(&opts.version, "version", false, "print version")
	cmd.PersistentFlags().
		StringVar(&opts.graphFormat, "format", opts.graphFormat, "graph output format: mermaid or json")
}

func normalizeArgs(args []string) []string {
	longFlags := map[string]bool{
		"-aliases":    true,
		"-dry-run":    true,
		"-env-file":   true,
		"-errors":     true,
		"-force":      true,
		"-failed":     true,
		"-format":     true,
		"-last":       true,
		"-log-only":   true,
		"-json":       true,
		"-profile":    true,
		"-runs-limit": true,
		"-since":      true,
		"-status":     true,
		"-target":     true,
		"-artifact":   true,
		"-var":        true,
		"-verbose":    true,
		"-version":    true,
		"-yes":        true,
	}
	normalized := make([]string, len(args))
	for index, arg := range args {
		if longFlags[arg] {
			normalized[index] = "-" + arg
			continue
		}
		normalized[index] = arg
	}
	return normalized
}

type variableFlags map[string]string

func (v variableFlags) String() string {
	if len(v) == 0 {
		return ""
	}
	keys := make([]string, 0, len(v))
	for key := range v {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+v[key])
	}
	return strings.Join(values, ",")
}

func (v variableFlags) Set(value string) error {
	name, variableValue, ok := strings.Cut(value, "=")
	if !ok || name == "" {
		return fmt.Errorf("expected name=value")
	}
	v[name] = variableValue
	return nil
}

func (v variableFlags) Type() string {
	return "name=value"
}
