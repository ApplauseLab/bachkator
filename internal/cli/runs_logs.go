package cli

import (
	"fmt"
	"io"

	"github.com/applauselab/bachkator/internal/query"
)

func runLogs(
	project *Project,
	deps Dependencies,
	opts *options,
	args []string,
	stdout io.Writer,
) error {
	if len(args) != 1 {
		return UsageErrorf("usage: bach logs <run-id>")
	}
	if deps.Logs == nil {
		return fmt.Errorf("log query dependency is not configured")
	}
	sections, err := deps.Logs(project, query.LogOptions{
		RunID:      args[0],
		Root:       projectRoot(project),
		Target:     opts.runsTarget,
		FailedOnly: opts.logsFailed,
		Last:       opts.logsLast,
		ErrorsOnly: opts.logsErrors,
	})
	if err != nil {
		return err
	}
	for _, section := range sections {
		if _, err := fmt.Fprintf(
			stdout,
			"==> %s (%s) %s <==\n",
			section.Target,
			section.Status,
			section.LogPath,
		); err != nil {
			return err
		}
		for _, line := range section.Lines {
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return err
			}
		}
	}
	return nil
}
