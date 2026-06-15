package cli

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/spf13/cobra"
)

type options struct {
	configPath           string
	variables            variableFlags
	profiles             []string
	listAliases          bool
	listGenerated        bool
	verbose              bool
	runsLimit            int
	runsTarget           string
	runsStatus           model.RunStatus
	runsSince            string
	artifactPath         string
	logsFailed           bool
	logsLast             int
	logsErrors           bool
	dryRun               bool
	force                bool
	yes                  bool
	envFile              string
	logOnly              bool
	jobs                 int
	json                 bool
	version              bool
	graphFormat          string
	initProvider         string
	factoryWorkflow      string
	factoryTitle         string
	factoryBody          string
	factoryBodyFile      string
	factoryPriority      model.Priority
	factoryStatus        model.Lifecycle
	factoryLabels        []string
	factoryDedupeKey     string
	factoryPlan          string
	factoryReason        string
	factoryPhase         string
	planParallelism      int
	planStopOn           string
	factoryPollInterval  time.Duration
	factoryRenewInterval time.Duration
	factoryLeaseTTL      time.Duration
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
		BoolVarP(&opts.verbose, "verbose", "v", false, "show extra target metadata and stream output for quiet targets")
	cmd.PersistentFlags().
		BoolVar(&opts.json, "json", false, "print machine-readable output when supported")
	cmd.PersistentFlags().BoolVar(&opts.version, "version", false, "print version")
}

func bindListFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().
		BoolVarP(&opts.listAliases, "aliases", "a", false, "include target aliases with list")
	cmd.Flags().
		BoolVar(&opts.listGenerated, "generated", false, "include generated policy nodes with list")
}

func bindExecutionFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().BoolVarP(&opts.dryRun, "dry-run", "d", false, "print commands without running them")
	cmd.Flags().BoolVar(&opts.force, "force", false, "run targets even when cached state is fresh")
	cmd.Flags().
		BoolVarP(&opts.yes, "yes", "y", false, "confirm execution of targets marked requires_confirmation")
	cmd.Flags().
		StringVar(&opts.envFile, "env-file", "", "load command environment from file instead of project .env")
	cmd.Flags().
		BoolVar(&opts.logOnly, "log-only", false, "suppress command output; keep progress on terminal and logs")
	cmd.Flags().IntVarP(&opts.jobs, "jobs", "j", opts.jobs, "maximum targets to run in parallel")
}

func bindPlanExecutionFlags(cmd *cobra.Command, opts *options) {
	bindExecutionFlags(cmd, opts)
	cmd.Flags().
		IntVar(&opts.planParallelism, "parallelism", 1, "maximum Plans to execute in parallel within a batch")
	cmd.Flags().
		StringVar(&opts.planStopOn, "stop-on", "failure", "stop batch on first failure: failure or never")
}

func bindRunsFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().
		IntVar(&opts.runsLimit, "runs-limit", opts.runsLimit, "maximum runs to list; use 0 for all")
	cmd.Flags().StringVar(&opts.runsTarget, "target", "", "filter runs or artifacts by target")
	cmd.Flags().
		StringVar((*string)(&opts.runsStatus), "status", "", "filter runs or artifacts by run status")
	cmd.Flags().
		StringVar(&opts.runsSince, "since", "", "filter runs or artifacts since a duration like 24h or an RFC3339 time")
	cmd.Flags().
		StringVar(&opts.artifactPath, "artifact", "", "filter runs or artifacts by artifact path substring")
}

func bindLogsFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().BoolVar(&opts.logsFailed, "failed", false, "show only failed target logs")
	cmd.Flags().IntVar(&opts.logsLast, "last", 0, "show only the last N log lines")
	cmd.Flags().BoolVar(&opts.logsErrors, "errors", false, "show only likely error lines")
}

func bindGraphFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().
		StringVar(&opts.graphFormat, "format", opts.graphFormat, "graph output format: mermaid or json")
}

func bindInitFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print files without writing them")
	cmd.Flags().
		StringVarP(&opts.initProvider, "provider", "p", "", "provider to use with init, such as opencode")
}

func bindFactorySubmitFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().StringVar(&opts.factoryWorkflow, "workflow", "", "factory workflow to use")
	cmd.Flags().StringVar(&opts.factoryTitle, "title", "", "factory work item title")
	cmd.Flags().StringVar(&opts.factoryBody, "body", "", "factory work item body text")
	cmd.Flags().
		StringVar(&opts.factoryBodyFile, "body-file", "", "project-relative factory work item body file")
	cmd.Flags().
		StringVar((*string)(&opts.factoryPriority), "priority", "", "factory work item priority")
	cmd.Flags().
		StringArrayVar(&opts.factoryLabels, "label", nil, "factory work item label; may be repeated")
	cmd.Flags().StringVar(&opts.factoryDedupeKey, "dedupe-key", "", "factory open-item dedupe key")
	cmd.Flags().StringVar(&opts.factoryPlan, "plan", "", "factory submitted plan reference")
}

func bindFactoryListFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().StringVar(&opts.factoryWorkflow, "workflow", "", "filter by workflow")
	cmd.Flags().StringVar((*string)(&opts.factoryStatus), "status", "", "filter by status")
}

func bindFactoryCancelFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().StringVar(&opts.factoryReason, "reason", "", "cancellation reason")
}

func bindFactoryApproveFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().StringVar(&opts.factoryPhase, "phase", "", "phase to approve")
	cmd.Flags().StringVar(&opts.factoryReason, "reason", "", "approval reason")
}

func bindFactoryStartFlags(cmd *cobra.Command, opts *options) {
	cmd.Flags().DurationVar(
		&opts.factoryPollInterval,
		"poll-interval",
		5*time.Second,
		"interval between daemon queue polls",
	)
	cmd.Flags().DurationVar(
		&opts.factoryRenewInterval,
		"renew-interval",
		10*time.Second,
		"interval between daemon lease renewals",
	)
	cmd.Flags().DurationVar(
		&opts.factoryLeaseTTL,
		"lease-ttl",
		30*time.Second,
		"duration until an unrenewed daemon lease expires",
	)
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
