package cli

import (
	"fmt"
	"path/filepath"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/query"
)

func inspectRun(
	project *Project,
	deps Dependencies,
	opts *options,
	runID string,
) (runInspection, error) {
	if deps.InspectRun == nil {
		return runInspection{}, fmt.Errorf("run inspection query dependency is not configured")
	}
	queried, err := deps.InspectRun(project, query.RunInspectOptions{RunID: runID})
	if err != nil {
		return runInspection{}, err
	}
	inspection := runInspection{
		RunID:           queried.RunID,
		RequestedTarget: queried.RequestedTarget,
		Status:          queried.Status,
		StartedAt:       queried.StartedAt.Format(timeFormat),
		LogDir:          queried.LogDir,
	}
	if !queried.FinishedAt.IsZero() {
		inspection.FinishedAt = queried.FinishedAt.Format(timeFormat)
	}
	for _, target := range queried.Targets {
		inspection.Targets = append(
			inspection.Targets,
			targetInspectionFromQuery(project, queried, target),
		)
	}
	for _, queriedFailure := range queried.FailedTargets {
		failure := targetFailureInspection{
			Target:    queriedFailure.Target,
			Status:    queriedFailure.Status,
			ExitCode:  queriedFailure.ExitCode,
			Operation: queriedFailure.Operation,
			LogPath:   queriedFailure.LogPath,
			Artifacts: append([]string(nil), queriedFailure.Artifacts...),
			Quality:   qualityFromQuery(queriedFailure.Quality),
		}
		failure.PreflightFailures = preflightFailuresForTarget(
			project,
			queriedFailure.Target,
			queriedFailure.Status,
			queriedFailure.Operation,
		)
		failure.MissingTools = missingToolsForTarget(
			project,
			queriedFailure.Target,
			queriedFailure.Status,
			queriedFailure.Operation,
		)
		failure.LogExcerpt = query.LogExcerpt(
			projectRoot(project),
			queriedFailure.LogPath,
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

func targetInspectionFromQuery(
	project *Project,
	run query.RunInspection,
	target query.TargetRunInspection,
) targetRunInspection {
	return targetRunInspection{
		Target:    target.Target,
		Status:    target.Status,
		Operation: target.Operation,
		LogPath:   target.LogPath,
		Artifacts: append([]string(nil), target.Artifacts...),
		Quality:   qualityFromQuery(target.Quality),
		AgentReports: agentReportsForTarget(
			projectRoot(project),
			run.RunID,
			run.LogDir,
			target.Target,
		),
		PolicyEvaluations: policyEvaluationsForTarget(
			projectRoot(project),
			run.RunID,
			run.LogDir,
			target.Target,
		),
	}
}

func qualityFromQuery(quality query.TargetQualityInspection) targetQualityInspection {
	out := targetQualityInspection{}
	for _, report := range quality.Reports {
		out.Reports = append(out.Reports, qualityReportInspection{
			Path:     report.Path,
			Kind:     report.Kind,
			Format:   report.Format,
			Status:   report.Status,
			Parsed:   report.Parsed,
			Metrics:  report.Metrics,
			Findings: report.Findings,
			Message:  report.Message,
		})
	}
	for _, gate := range quality.FailedGates {
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
	status model.RunStatus,
	operation string,
) []preflightFailureInspection {
	if status != model.RunStatusPreflightFailed || operation != "credential/session preflight" {
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
	status model.RunStatus,
	operation string,
) []toolFailureInspection {
	if status != model.RunStatusFailed || operation != "required tool check" {
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

func projectRoot(project *Project) string {
	if project == nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(project.StatePath))
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
