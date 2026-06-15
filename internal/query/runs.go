package query

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
	statestore "github.com/applauselab/bachkator/internal/state"
)

type RunInspectOptions struct {
	RunID string
}

type LogOptions struct {
	RunID      string
	Root       string
	Target     string
	FailedOnly bool
	Last       int
	ErrorsOnly bool
}

type LogSection struct {
	Target  string
	Status  model.RunStatus
	LogPath string
	Lines   []string
}

type RunInspection struct {
	RunID           string
	RequestedTarget string
	Status          model.RunStatus
	StartedAt       time.Time
	FinishedAt      time.Time
	LogDir          string
	Targets         []TargetRunInspection
	FailedTargets   []TargetFailureInspection
}

type TargetRunInspection struct {
	Target    string
	Status    model.RunStatus
	Operation string
	LogPath   string
	ExitCode  *int
	Artifacts []string
	Quality   TargetQualityInspection
	Raw       statestore.TargetRunRecord
}

type TargetFailureInspection struct {
	Target    string
	Status    model.RunStatus
	ExitCode  *int
	Operation string
	LogPath   string
	Artifacts []string
	Quality   TargetQualityInspection
	Raw       statestore.TargetRunRecord
}

type TargetQualityInspection struct {
	Reports     []QualityReportInspection
	FailedGates []QualityGateInspection
}

type QualityReportInspection struct {
	Path     string
	Kind     string
	Format   string
	Status   model.RunStatus
	Parsed   bool
	Metrics  int
	Findings int
	Message  string
}

type QualityGateInspection struct {
	Metric    string
	Op        string
	Threshold float64
	Actual    float64
	Message   string
}

type RunInspectStore interface {
	Load() (*statestore.State, error)
	QualityReportsForRun(runID string) ([]quality.Report, error)
	QualityGateResultsForRun(runID string) ([]quality.GateResult, error)
}

type LogStore interface {
	Load() (*statestore.State, error)
}

func InspectRun(store RunInspectStore, opts RunInspectOptions) (RunInspection, error) {
	snapshot, err := store.Load()
	if err != nil {
		return RunInspection{}, err
	}
	var selected *statestore.RunRecord
	for index := range snapshot.Runs {
		if snapshot.Runs[index].ID == opts.RunID {
			selected = &snapshot.Runs[index]
			break
		}
	}
	if selected == nil {
		return RunInspection{}, bacherr.NotFoundf("run %q", opts.RunID)
	}
	reports, err := store.QualityReportsForRun(opts.RunID)
	if err != nil {
		return RunInspection{}, err
	}
	gates, err := store.QualityGateResultsForRun(opts.RunID)
	if err != nil {
		return RunInspection{}, err
	}
	inspection := RunInspection{
		RunID:           selected.ID,
		RequestedTarget: selected.Target,
		Status:          selected.Status,
		StartedAt:       selected.StartedAt,
		FinishedAt:      selected.FinishedAt,
		LogDir:          selected.LogDir,
	}
	for _, targetName := range sortedTargetRunNames(selected.Targets) {
		targetRun := selected.Targets[targetName]
		target := targetInspectionForRun(*selected, reports, gates, targetName, targetRun)
		inspection.Targets = append(inspection.Targets, target)
		if !failedStatus(targetRun.Status) {
			continue
		}
		inspection.FailedTargets = append(inspection.FailedTargets, TargetFailureInspection{
			Target:    targetName,
			Status:    targetRun.Status,
			ExitCode:  targetRun.ExitCode,
			Operation: targetRun.Operation,
			LogPath:   targetRun.LogPath,
			Artifacts: target.Artifacts,
			Quality:   target.Quality,
			Raw:       targetRun,
		})
	}
	return inspection, nil
}

func Logs(store LogStore, opts LogOptions) ([]LogSection, error) {
	snapshot, err := store.Load()
	if err != nil {
		return nil, err
	}
	var selected *statestore.RunRecord
	for index := range snapshot.Runs {
		if snapshot.Runs[index].ID == opts.RunID {
			selected = &snapshot.Runs[index]
			break
		}
	}
	if selected == nil {
		return nil, bacherr.NotFoundf("run %q", opts.RunID)
	}
	sections := []LogSection{}
	for _, targetName := range sortedTargetRunNames(selected.Targets) {
		targetRun := selected.Targets[targetName]
		if opts.Target != "" && opts.Target != targetName {
			continue
		}
		if opts.FailedOnly && !failedStatus(targetRun.Status) {
			continue
		}
		lines := LogExcerpt(opts.Root, targetRun.LogPath, opts.Last, opts.ErrorsOnly)
		if len(lines) == 0 {
			continue
		}
		sections = append(sections, LogSection{
			Target:  targetName,
			Status:  targetRun.Status,
			LogPath: targetRun.LogPath,
			Lines:   lines,
		})
	}
	return sections, nil
}

func LogExcerpt(root string, logPath string, last int, errorsOnly bool) []string {
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

func targetInspectionForRun(
	run statestore.RunRecord,
	reports []quality.Report,
	gates []quality.GateResult,
	targetName string,
	targetRun statestore.TargetRunRecord,
) TargetRunInspection {
	return TargetRunInspection{
		Target:    targetName,
		Status:    targetRun.Status,
		Operation: targetRun.Operation,
		LogPath:   targetRun.LogPath,
		ExitCode:  targetRun.ExitCode,
		Artifacts: artifactsForTarget(run, targetName),
		Quality:   qualityForTarget(reports, gates, targetName),
		Raw:       targetRun,
	}
}

func sortedTargetRunNames(targets map[string]statestore.TargetRunRecord) []string {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func failedStatus(status model.RunStatus) bool {
	return status == model.RunStatusFailed ||
		status == model.RunStatusQualityFailed ||
		status == model.RunStatusPreflightFailed
}

func readLogLines(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	lines := []string{}
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

func artifactsForTarget(run statestore.RunRecord, target string) []string {
	artifacts := []string{}
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
	reports []quality.Report,
	gates []quality.GateResult,
	target string,
) TargetQualityInspection {
	out := TargetQualityInspection{}
	for _, report := range reports {
		if report.Target != target {
			continue
		}
		out.Reports = append(out.Reports, QualityReportInspection{
			Path:     report.Path,
			Kind:     report.Kind,
			Format:   report.Format,
			Status:   report.Status,
			Parsed:   report.Status == model.RunStatusSuccess,
			Metrics:  len(report.Metrics),
			Findings: len(report.Findings),
			Message:  report.Message,
		})
	}
	for _, gate := range gates {
		if gate.Target != target || gate.Status != "failed" {
			continue
		}
		out.FailedGates = append(out.FailedGates, QualityGateInspection{
			Metric:    gate.Metric,
			Op:        gate.Op,
			Threshold: gate.Threshold,
			Actual:    gate.Actual,
			Message:   gate.Message,
		})
	}
	return out
}
