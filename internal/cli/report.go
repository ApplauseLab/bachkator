package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/applauselab/bachkator/internal/quality/agentreport"
	"github.com/spf13/cobra"
)

func newReportCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Write agent quality report artifacts",
		Long:  "Write agent quality report artifacts. Agent reports use the bach.agent_report.v1 schema.",
	}

	defaults := &agentreport.Defaults{Env: envMap()}
	bindReportDefaults := func(c *cobra.Command, includeName bool) {
		c.Flags().StringVar(&defaults.Path, "path", "", "report path")
		c.Flags().StringVar(&defaults.Role, "role", "", "agent role")
		c.Flags().StringVar(&defaults.Summary, "summary", "", "report summary")
		c.Flags().BoolVar(
			&defaults.AllowExternalPath,
			"allow-external-path",
			false,
			"allow writing the report outside the workspace",
		)
		if includeName {
			c.Flags().StringVar(&defaults.Name, "name", "", "agent name")
		}
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize an agent report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := agentreport.WriteInit(*defaults)
			return printReportPath(stdout, path, err)
		},
	}
	bindReportDefaults(initCmd, true)
	cmd.AddCommand(initCmd)

	findingCmd := &cobra.Command{
		Use:   "finding",
		Short: "Append a finding to an agent report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			finding, err := parseFindingFlags(cmd, stdin)
			if err != nil {
				return err
			}
			path, err := agentreport.AppendFinding(*defaults, finding)
			return printReportPath(stdout, path, err)
		},
	}
	bindReportDefaults(findingCmd, true)
	findingCmd.Flags().String("kind", "", "finding kind")
	findingCmd.Flags().String("severity", "", "finding severity")
	findingCmd.Flags().String("rule", "", "finding rule")
	findingCmd.Flags().String("message", "", "finding message")
	findingCmd.Flags().String("file", "", "finding file")
	findingCmd.Flags().Int("line", 0, "finding line")
	findingCmd.Flags().Float64("duration-ms", 0, "finding duration in milliseconds")
	findingCmd.Flags().Bool("stdin", false, "read finding JSON from stdin")
	cmd.AddCommand(findingCmd)

	metricCmd := &cobra.Command{
		Use:   "metric",
		Short: "Append a metric to an agent report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			metric, err := parseMetricFlags(cmd)
			if err != nil {
				return err
			}
			path, err := agentreport.AppendMetric(*defaults, metric)
			return printReportPath(stdout, path, err)
		},
	}
	bindReportDefaults(metricCmd, false)
	metricCmd.Flags().String("name", "", "metric name")
	metricCmd.Flags().String("value", "", "metric value")
	metricCmd.Flags().String("scope", "", "metric scope")
	metricCmd.Flags().String("unit", "", "metric unit")
	cmd.AddCommand(metricCmd)

	statusCmd := &cobra.Command{
		Use:   "status <success|failed>",
		Short: "Update agent report status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := agentreport.UpdateStatus(*defaults, args[0])
			return printReportPath(stdout, path, err)
		},
	}
	bindReportDefaults(statusCmd, true)
	cmd.AddCommand(statusCmd)

	return cmd
}

func parseFindingFlags(cmd *cobra.Command, stdin io.Reader) (agentreport.Finding, error) {
	useStdin, err := cmd.Flags().GetBool("stdin")
	if err != nil {
		return agentreport.Finding{}, err
	}
	if useStdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return agentreport.Finding{}, err
		}
		return agentreport.DecodeFindingStrict(data)
	}

	finding := agentreport.Finding{}
	finding.Kind, _ = cmd.Flags().GetString("kind")
	finding.Severity, _ = cmd.Flags().GetString("severity")
	finding.Rule, _ = cmd.Flags().GetString("rule")
	finding.Message, _ = cmd.Flags().GetString("message")
	finding.File, _ = cmd.Flags().GetString("file")
	finding.Line, _ = cmd.Flags().GetInt("line")
	finding.DurationMS, _ = cmd.Flags().GetFloat64("duration-ms")
	return finding, nil
}

func parseMetricFlags(cmd *cobra.Command) (agentreport.Metric, error) {
	metric := agentreport.Metric{}
	metric.Name, _ = cmd.Flags().GetString("name")
	valueText, _ := cmd.Flags().GetString("value")
	metric.Scope, _ = cmd.Flags().GetString("scope")
	metric.Unit, _ = cmd.Flags().GetString("unit")

	if valueText == "" {
		return metric, fmt.Errorf("metric value is required")
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil {
		return metric, fmt.Errorf("metric value must be a number: %w", err)
	}
	metric.Value = value
	return metric, nil
}

func printReportPath(stdout io.Writer, path string, err error) error {
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "wrote agent report %s\n", path)
	return err
}

func envMap() map[string]string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}
