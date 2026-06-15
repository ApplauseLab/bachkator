package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newRunsCommand(
	deps Dependencies,
	opts *options,
	stdout io.Writer,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List recorded runs",
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListRuns(projectFromContext(cmd.Context()), deps, opts, stdout)
		},
	}
	bindRunsFlags(listCmd, opts)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "inspect <run-id>",
		Short: "Inspect a recorded run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspectRun(projectFromContext(cmd.Context()), deps, opts, args[0], stdout)
		},
	})
	return cmd
}

func runInspectRun(
	project *Project,
	deps Dependencies,
	opts *options,
	runID string,
	stdout io.Writer,
) error {
	inspection, err := inspectRun(project, deps, opts, runID)
	if err != nil {
		return err
	}
	if opts.json {
		data, err := json.MarshalIndent(inspection, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s\n", data)
		return err
	}
	return formatRunInspection(stdout, inspection)
}
