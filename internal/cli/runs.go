package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	statestore "github.com/applause/bachkator/internal/state"
)

const timeFormat = "2006-01-02T15:04:05Z07:00"

type runInspection struct {
	RunID           string                    `json:"run_id"`
	RequestedTarget string                    `json:"requested_target"`
	Status          string                    `json:"status"`
	StartedAt       string                    `json:"started_at"`
	FinishedAt      string                    `json:"finished_at,omitempty"`
	LogDir          string                    `json:"log_dir"`
	FailedTargets   []targetFailureInspection `json:"failed_targets"`
	SuggestedFixes  []string                  `json:"suggested_fixes"`
}

type targetFailureInspection struct {
	Target            string                       `json:"target"`
	Status            string                       `json:"status"`
	ExitCode          *int                         `json:"exit_code,omitempty"`
	Operation         string                       `json:"operation,omitempty"`
	LogPath           string                       `json:"log_path,omitempty"`
	Artifacts         []string                     `json:"artifacts,omitempty"`
	Quality           targetQualityInspection      `json:"quality"`
	PreflightFailures []preflightFailureInspection `json:"preflight_failures,omitempty"`
	MissingTools      []toolFailureInspection      `json:"missing_tools,omitempty"`
	LogExcerpt        []string                     `json:"log_excerpt,omitempty"`
}

type targetQualityInspection struct {
	Reports     []qualityReportInspection `json:"reports,omitempty"`
	FailedGates []qualityGateInspection   `json:"failed_gates,omitempty"`
}

type qualityReportInspection struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Format   string `json:"format,omitempty"`
	Status   string `json:"status"`
	Parsed   bool   `json:"parsed"`
	Metrics  int    `json:"metrics"`
	Findings int    `json:"findings"`
	Message  string `json:"message,omitempty"`
}

type qualityGateInspection struct {
	Metric    string  `json:"metric"`
	Op        string  `json:"op"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	Message   string  `json:"message"`
}

type preflightFailureInspection struct {
	Name   string `json:"name"`
	Kind   string `json:"kind,omitempty"`
	Fix    string `json:"fix,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type toolFailureInspection struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Fix     string `json:"fix,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func runRuns(project *Project, opts *options, args []string, stdout io.Writer) error {
	if len(args) > 0 && args[0] == "inspect" {
		if len(args) != 2 {
			return fmt.Errorf("usage: bach runs inspect <run-id>")
		}
		return runInspectRun(project, opts, args[1], stdout)
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown runs subcommand %q", args[0])
	}
	return runListRuns(project, opts, stdout)
}

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

func runInspectRun(project *Project, opts *options, runID string, stdout io.Writer) error {
	inspection, err := inspectRun(project, opts, runID)
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

func inspectRun(project *Project, opts *options, runID string) (runInspection, error) {
	store := statestore.NewStore(project.StatePath)
	snapshot, err := store.Load()
	if err != nil {
		return runInspection{}, err
	}
	var selected *statestore.RunRecord
	for index := range snapshot.Runs {
		if snapshot.Runs[index].ID == runID {
			selected = &snapshot.Runs[index]
			break
		}
	}
	if selected == nil {
		return runInspection{}, fmt.Errorf("run %q not found", runID)
	}
	reports, err := store.QualityReportsForRun(runID)
	if err != nil {
		return runInspection{}, err
	}
	gates, err := store.QualityGateResultsForRun(runID)
	if err != nil {
		return runInspection{}, err
	}
	inspection := runInspection{
		RunID:           selected.ID,
		RequestedTarget: selected.Target,
		Status:          selected.Status,
		StartedAt:       selected.StartedAt.Format(timeFormat),
		LogDir:          selected.LogDir,
	}
	if !selected.FinishedAt.IsZero() {
		inspection.FinishedAt = selected.FinishedAt.Format(timeFormat)
	}
	for _, targetName := range sortedTargetRunNames(selected.Targets) {
		targetRun := selected.Targets[targetName]
		if !failedStatus(targetRun.Status) {
			continue
		}
		failure := targetFailureInspection{
			Target:    targetName,
			Status:    targetRun.Status,
			ExitCode:  targetRun.ExitCode,
			Operation: targetRun.Operation,
			LogPath:   targetRun.LogPath,
			Artifacts: artifactsForTarget(*selected, targetName),
		}
		failure.Quality = qualityForTarget(reports, gates, targetName)
		failure.PreflightFailures = preflightFailuresForTarget(project, targetName, targetRun)
		failure.MissingTools = missingToolsForTarget(project, targetName, targetRun)
		failure.LogExcerpt = logExcerpt(
			projectRoot(project),
			targetRun.LogPath,
			opts.logsLast,
			opts.logsErrors,
		)
		for _, preflight := range failure.PreflightFailures {
			if preflight.Fix != "" {
				inspection.SuggestedFixes = append(inspection.SuggestedFixes, preflight.Fix)
			}
		}
		for _, tool := range failure.MissingTools {
			if tool.Fix != "" {
				inspection.SuggestedFixes = append(inspection.SuggestedFixes, tool.Fix)
			}
		}
		inspection.FailedTargets = append(inspection.FailedTargets, failure)
	}
	inspection.SuggestedFixes = uniqueStrings(inspection.SuggestedFixes)
	return inspection, nil
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

func sortedTargetRunNames(targets map[string]statestore.TargetRunRecord) []string {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func failedStatus(status string) bool {
	return status == "failed" || status == "quality-failed" || status == "preflight-failed"
}

func artifactsForTarget(run statestore.RunRecord, target string) []string {
	var artifacts []string
	for _, artifact := range run.Artifacts {
		if artifact.Target != target {
			continue
		}
		if artifact.Path != "" {
			artifacts = append(artifacts, artifact.Path)
		} else if artifact.Value != "" {
			artifacts = append(artifacts, artifact.Value)
		}
	}
	return artifacts
}

func qualityForTarget(
	reports []statestore.QualityReport,
	gates []statestore.QualityGateResult,
	target string,
) targetQualityInspection {
	var out targetQualityInspection
	for _, report := range reports {
		if report.Target != target {
			continue
		}
		out.Reports = append(out.Reports, qualityReportInspection{
			Path:     report.Path,
			Kind:     report.Kind,
			Format:   report.Format,
			Status:   report.Status,
			Parsed:   report.Status == "success",
			Metrics:  len(report.Metrics),
			Findings: len(report.Findings),
			Message:  report.Message,
		})
	}
	for _, gate := range gates {
		if gate.Target != target || gate.Status != "failed" {
			continue
		}
		out.FailedGates = append(out.FailedGates, qualityGateInspection{
			Metric:    gate.Metric,
			Op:        gate.Op,
			Threshold: gate.Threshold,
			Actual:    gate.Actual,
			Message:   gate.Message,
		})
	}
	return out
}

func preflightFailuresForTarget(
	project *Project,
	target string,
	targetRun statestore.TargetRunRecord,
) []preflightFailureInspection {
	if targetRun.Status != "preflight-failed" ||
		targetRun.Operation != "credential/session preflight" {
		return nil
	}
	targetSpec := project.Targets[target]
	if targetSpec == nil {
		return nil
	}
	var failures []preflightFailureInspection
	for _, preflight := range targetSpec.Spec.Runtime.Preflights {
		failures = append(failures, preflightFailureInspection{
			Name: preflight.Label(),
			Kind: preflight.Kind,
			Fix:  preflight.Fix,
		})
	}
	return failures
}

func missingToolsForTarget(
	project *Project,
	target string,
	targetRun statestore.TargetRunRecord,
) []toolFailureInspection {
	if targetRun.Status != "failed" || targetRun.Operation != "required tool check" {
		return nil
	}
	targetSpec := project.Targets[target]
	if targetSpec == nil {
		return nil
	}
	var failures []toolFailureInspection
	for _, tool := range targetSpec.Spec.Runtime.Tools {
		failures = append(failures, toolFailureInspection{
			Name:    tool.Name,
			Version: tool.Version,
			Fix:     tool.Fix,
		})
	}
	return failures
}

func logExcerpt(root string, logPath string, last int, errorsOnly bool) []string {
	if logPath == "" {
		return nil
	}
	path := logPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	lines := readLogLines(path)
	if errorsOnly {
		filtered := lines[:0]
		for _, line := range lines {
			if likelyErrorLine(line) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}
	if last <= 0 {
		last = 20
	}
	if len(lines) > last {
		lines = lines[len(lines)-last:]
	}
	return lines
}

func projectRoot(project *Project) string {
	if project == nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(project.StatePath))
}

func readLogLines(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func likelyErrorLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "error") || strings.Contains(lower, "failed") ||
		strings.Contains(lower, "fail") || strings.Contains(lower, "panic")
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
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

func runLogs(project *Project, opts *options, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: bach logs <run-id>")
	}
	snapshot, err := statestore.NewStore(project.StatePath).Load()
	if err != nil {
		return err
	}
	var selected *statestore.RunRecord
	for index := range snapshot.Runs {
		if snapshot.Runs[index].ID == args[0] {
			selected = &snapshot.Runs[index]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("run %q not found", args[0])
	}
	for _, targetName := range sortedTargetRunNames(selected.Targets) {
		targetRun := selected.Targets[targetName]
		if opts.runsTarget != "" && opts.runsTarget != targetName {
			continue
		}
		if opts.logsFailed && !failedStatus(targetRun.Status) {
			continue
		}
		lines := logExcerpt(projectRoot(project), targetRun.LogPath, opts.logsLast, opts.logsErrors)
		if len(lines) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(
			stdout,
			"==> %s (%s) %s <==\n",
			targetName,
			targetRun.Status,
			targetRun.LogPath,
		); err != nil {
			return err
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return err
			}
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
