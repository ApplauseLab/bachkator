package cli

import (
	"fmt"
	"io"

	"github.com/applauselab/bachkator/internal/query"
)

func runListRuns(project *Project, deps Dependencies, opts *options, stdout io.Writer) error {
	if deps.ListRuns == nil {
		return fmt.Errorf("run list query dependency is not configured")
	}
	since, err := parseSince(opts.runsSince)
	if err != nil {
		return err
	}
	runs, err := deps.ListRuns(project, query.RunListOptions{
		Target:       opts.runsTarget,
		Status:       opts.runsStatus,
		Since:        since,
		ArtifactPath: opts.artifactPath,
		Limit:        opts.runsLimit,
	})
	if err != nil {
		return err
	}
	for _, run := range runs {
		finished := "-"
		if !run.FinishedAt.IsZero() {
			finished = run.FinishedAt.Format(timeFormat)
		}
		mode := "run"
		if run.DryRun {
			mode = "dry-run"
		}
		if run.Force {
			mode += ",force"
		}
		if _, err := fmt.Fprintf(
			stdout,
			"%s %-8s %-12s %-24s started=%s finished=%s logs=%s\n",
			run.ID,
			run.Status,
			mode,
			run.Target,
			run.StartedAt.Format(timeFormat),
			finished,
			run.LogDir,
		); err != nil {
			return err
		}
	}
	return nil
}

func formatRunInspection(stdout io.Writer, inspection runInspection) error {
	if _, err := fmt.Fprintf(
		stdout,
		"run %s %s target=%s\n",
		inspection.RunID,
		inspection.Status,
		inspection.RequestedTarget,
	); err != nil {
		return err
	}
	if len(inspection.FailedTargets) == 0 {
		_, err := fmt.Fprintln(stdout, "failed targets: none")
		return err
	}
	if _, err := fmt.Fprintln(stdout, "failed targets:"); err != nil {
		return err
	}
	for _, target := range inspection.FailedTargets {
		exit := ""
		if target.ExitCode != nil {
			exit = fmt.Sprintf(" exit=%d", *target.ExitCode)
		}
		if _, err := fmt.Fprintf(
			stdout,
			"- %s status=%s%s log=%s\n",
			target.Target,
			target.Status,
			exit,
			target.LogPath,
		); err != nil {
			return err
		}
		for _, report := range target.Quality.Reports {
			if _, err := fmt.Fprintf(
				stdout,
				"  quality report %s %s metrics=%d findings=%d\n",
				report.Path,
				report.Status,
				report.Metrics,
				report.Findings,
			); err != nil {
				return err
			}
		}
		for _, gate := range target.Quality.FailedGates {
			if _, err := fmt.Fprintf(
				stdout,
				"  quality gate failed: %s\n",
				gate.Message,
			); err != nil {
				return err
			}
		}
	}
	if len(inspection.SuggestedFixes) > 0 {
		if _, err := fmt.Fprintln(stdout, "suggested fixes:"); err != nil {
			return err
		}
		for _, fix := range inspection.SuggestedFixes {
			if _, err := fmt.Fprintf(stdout, "- %s\n", fix); err != nil {
				return err
			}
		}
	}
	return nil
}
