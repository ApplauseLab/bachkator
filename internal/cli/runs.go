package cli

import (
	"fmt"
	"io"
	"time"

	statestore "github.com/applause/bachkator/internal/state"
)

const timeFormat = "2006-01-02T15:04:05Z07:00"

func runListRuns(project *Project, opts *options, stdout io.Writer) error {
	since, err := parseSince(opts.runsSince)
	if err != nil {
		return err
	}
	runs, err := statestore.NewStore(project.StatePath).
		ListRuns(statestore.RunQuery{Target: opts.runsTarget, Status: opts.runsStatus, Since: since, ArtifactPath: opts.artifactPath, Limit: opts.runsLimit})
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

func runListArtifacts(project *Project, opts *options, args []string, stdout io.Writer) error {
	since, err := parseSince(opts.runsSince)
	if err != nil {
		return err
	}
	runID := ""
	if len(args) > 0 {
		runID = args[0]
	}
	artifacts, err := statestore.NewStore(project.StatePath).
		ListArtifacts(statestore.ArtifactQuery{RunID: runID, Target: opts.runsTarget, Status: opts.runsStatus, Since: since, Path: opts.artifactPath, Limit: opts.runsLimit})
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		location := artifact.Path
		if location == "" {
			location = artifact.Value
		}
		if _, err := fmt.Fprintf(
			stdout,
			"%s %-12s %-24s %-12s %s\n",
			artifact.RunID,
			artifact.Kind,
			artifact.Target,
			artifact.CreatedAt.Format(timeFormat),
			location,
		); err != nil {
			return err
		}
	}
	return nil
}

func parseSince(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return time.Now().UTC().Add(-duration), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"invalid --since %q: use a duration like 24h or an RFC3339 time",
			value,
		)
	}
	return parsed, nil
}
